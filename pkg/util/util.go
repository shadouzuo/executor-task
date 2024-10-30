package util

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/btcsuite/btcd/wire"
	go_coin_btc "github.com/pefish/go-coin-btc"
	go_decimal "github.com/pefish/go-decimal"
	i_logger "github.com/pefish/go-interface/i-logger"
	t_mysql "github.com/pefish/go-interface/t-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
)

func SendBtc(
	logger i_logger.ILogger,
	btcWallet *go_coin_btc.Wallet,
	priv string,
	fromAddress string,
	toAddr string,
	amount float64,
) (
	txId string,
	msgTx *wire.MsgTx,
	spentUtxos []constant.UTXO,
	newUtxos map[string][]*go_coin_btc.OutPoint,
	err error,
) {

	err = btcWallet.AddAccountByPrivKey(priv)
	if err != nil {
		return "", nil, nil, nil, err
	}

	feeRate, err := btcWallet.RpcClient.EstimateSmartFee()
	if err != nil {
		return "", nil, nil, nil, err
	}

	spentUtxos, err = SelectUtxos(fromAddress, amount, 0.001)
	if err != nil {
		return "", nil, nil, nil, err
	}

	outPoints := make([]*go_coin_btc.OutPoint, 0)
	for _, btcUtxo := range spentUtxos {
		outPoints = append(outPoints, &go_coin_btc.OutPoint{
			Hash:  btcUtxo.TxId,
			Index: int(btcUtxo.Index),
		})
	}
	if len(outPoints) == 0 {
		return "", nil, nil, nil, errors.New("Balance not enough. no utxo")
	}

	logger.InfoF("Build tx...")
	msgTx, newUtxos, _, err = btcWallet.BuildTx(
		outPoints,
		fromAddress,
		toAddr,
		amount,
		go_decimal.Decimal.MustStart(feeRate).RoundDown(0).MustEndForInt64(),
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
	notFound, err := global.MysqlInstance.SelectFirst(
		&addrDb,
		&t_mysql.SelectParams{
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
	_, err = global.MysqlInstance.Update(
		&t_mysql.UpdateParams{
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
	notFound, err := global.MysqlInstance.SelectFirst(
		&addrDb,
		&t_mysql.SelectParams{
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

func CheckUnConfirmedCountAndWait(logger i_logger.ILogger, task *constant.Task) error {
	count, err := global.MysqlInstance.Count(
		&t_mysql.CountParams{
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
	if count >= 20 {
		logger.InfoF("UnConfirmed tx count >= 10, just wait.")
		time.Sleep(10 * time.Second)
		err = CheckUnConfirmedCountAndWait(logger, task)
		if err != nil {
			return err
		}
	}
	return nil
}
