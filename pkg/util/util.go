package util

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/btcsuite/btcd/wire"
	go_coin_btc "github.com/pefish/go-coin-btc"
	go_logger "github.com/pefish/go-logger"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
)

func SendBtc(
	logger go_logger.InterfaceLogger,
	btcWallet *go_coin_btc.Wallet,
	priv string,
	fromAddress string,
	toAddr string,
	amount float64,
) (
	txId string,
	msgTx *wire.MsgTx,
	spentUtxos []constant.UTXO,
	newUtxos []*go_coin_btc.UTXO,
	err error,
) {
	feeRate, err := btcWallet.RpcClient.EstimateSmartFee()
	if err != nil {
		return "", nil, nil, nil, err
	}

	spentUtxos, err = SelectUtxos(fromAddress, amount, 0.001)
	if err != nil {
		return "", nil, nil, nil, err
	}

	outPointWithPrivs := make([]*go_coin_btc.UTXOWithPriv, 0)
	for _, btcUtxo := range spentUtxos {
		outPointWithPrivs = append(outPointWithPrivs, &go_coin_btc.UTXOWithPriv{
			Utxo: go_coin_btc.UTXO{
				TxId:  btcUtxo.TxId,
				Index: btcUtxo.Index,
			},
			Priv: priv,
		})
	}
	if len(outPointWithPrivs) == 0 {
		return "", nil, nil, nil, errors.New("Balance not enough. no utxo")
	}

	logger.InfoF("Build tx...")
	msgTx, newUtxos, _, err = btcWallet.BuildTx(
		outPointWithPrivs,
		fromAddress,
		toAddr,
		amount,
		feeRate*1.1,
	)
	if err != nil {
		return "", nil, nil, nil, err
	}

	// 发送交易
	logger.InfoF("Send tx...")
	txId, err = btcWallet.RpcClient.SendMsgTx(msgTx)
	if err != nil {
		return "", nil, nil, nil, err
	}

	return txId, msgTx, spentUtxos, newUtxos, nil
}

func UpdateUtxo(
	address string,
	spentUtxos []constant.UTXO,
	newUtxos []constant.UTXO,
) error {
	var addrDb constant.BtcAddress
	notFound, err := go_mysql.MysqlInstance.SelectFirst(
		&addrDb,
		&go_mysql.SelectParams{
			TableName: "btc_address",
			Select:    "*",
			Where: map[string]interface{}{
				"address": address,
			},
		},
	)
	if err != nil {
		return err
	}
	if notFound {
		return errors.New("Address not found.")
	}
	addressUtxos := make([]constant.UTXO, 0)
	if addrDb.Utxos != nil {
		err = json.Unmarshal([]byte(*addrDb.Utxos), &addressUtxos)
		if err != nil {
			return err
		}
	}

	newAddressUtxos := make([]constant.UTXO, 0)
	// 剔除用掉了的
	for _, addressUtxo := range addressUtxos {
		spent := false
		for _, spentUtxo := range spentUtxos {
			if strings.EqualFold(spentUtxo.TxId, addressUtxo.TxId) && spentUtxo.Index == addressUtxo.Index {
				spent = true
				break
			}
		}
		if !spent {
			newAddressUtxos = append(newAddressUtxos, addressUtxo)
		}
	}

	// 添加新的
	newAddressUtxos = append(newAddressUtxos, newUtxos...)

	// 更新
	b, _ := json.Marshal(newAddressUtxos)
	_, err = go_mysql.MysqlInstance.Update(
		&go_mysql.UpdateParams{
			TableName: "btc_address",
			Update: map[string]interface{}{
				"utxos": string(b),
			},
			Where: map[string]interface{}{
				"address": address,
			},
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func SelectUtxos(
	address string,
	targetAmount float64,
	estimateFee float64,
) ([]constant.UTXO, error) {
	var addrDb constant.BtcAddress
	notFound, err := go_mysql.MysqlInstance.SelectFirst(
		&addrDb,
		&go_mysql.SelectParams{
			TableName: "btc_address",
			Select:    "*",
			Where: map[string]interface{}{
				"address": address,
			},
		},
	)
	if err != nil {
		return nil, err
	}
	if notFound {
		return nil, errors.New("Address not found.")
	}
	addressUtxos := make([]constant.UTXO, 0)
	if addrDb.Utxos == nil {
		return nil, errors.New("No utxos.")
	}
	err = json.Unmarshal([]byte(*addrDb.Utxos), &addressUtxos)
	if err != nil {
		return nil, err
	}

	needAmount := targetAmount + estimateFee

	results := make([]constant.UTXO, 0)
	sum := 0.0
	for _, addressUtxo := range addressUtxos {
		sum += addressUtxo.Value
		results = append(results, addressUtxo)
		if sum >= needAmount {
			break
		}
	}

	return results, nil

}

func CheckUnConfirmedCountAndWait(logger go_logger.InterfaceLogger, task *constant.Task) error {
	count, err := go_mysql.MysqlInstance.Count(
		&go_mysql.CountParams{
			TableName: "btc_tx",
			Where: map[string]interface{}{
				"task_id": task.Id,
				"confirm": 0,
			},
		},
	)
	if err != nil {
		return err
	}
	if count >= 10 {
		logger.InfoF("UnConfirmed tx count >= 10, just wait.")
		time.Sleep(10 * time.Second)
		err = CheckUnConfirmedCountAndWait(logger, task)
		if err != nil {
			return err
		}
	}
	return nil
}
