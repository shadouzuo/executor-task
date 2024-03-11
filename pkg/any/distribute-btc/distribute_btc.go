package distribute_btc

import (
	"strings"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/pkg/errors"

	go_best_type "github.com/pefish/go-best-type"
	go_coin_btc "github.com/pefish/go-coin-btc"
	go_crypto "github.com/pefish/go-crypto"
	go_decimal "github.com/pefish/go-decimal"
	go_format "github.com/pefish/go-format"
	go_mysql "github.com/pefish/go-mysql"
	go_random "github.com/pefish/go-random"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/db"
	"github.com/shadouzuo/executor-task/pkg/global"
	"github.com/shadouzuo/executor-task/pkg/util"
)

type DistributeBtcType struct {
	go_best_type.BaseBestType
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type Config struct {
	Priv             string   `json:"priv"`
	Amount           string   `json:"amount"`
	SelectAddressSql []string `json:"select_address_sql"`
	BtcNodeUrl       string   `json:"btc_node_url"`
}

type ActionTypeData struct {
	Task *db.Task
}

func New(name string) *DistributeBtcType {
	t := &DistributeBtcType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *DistributeBtcType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	task := ask.Data.(ActionTypeData).Task

	newStatus, err := p.doLoop(exitChan, task)
	if err != nil {
		p.Logger().Error(err)
		_, err1 := go_mysql.MysqlInstance.Update(
			&go_mysql.UpdateParams{
				TableName: "task",
				Update: map[string]interface{}{
					"status": newStatus,
					"mark":   err.Error(),
				},
				Where: map[string]interface{}{
					"id": task.Id,
				},
			},
		)
		if err1 != nil {
			p.Logger().Error(err1)
			return err1
		}
		return err
	}
	_, err = go_mysql.MysqlInstance.Update(
		&go_mysql.UpdateParams{
			TableName: "task",
			Update: map[string]interface{}{
				"status": newStatus,
			},
			Where: map[string]interface{}{
				"id": task.Id,
			},
		},
	)
	if err != nil {
		p.Logger().Error(err)
		return err
	}
	return nil
}

func (p *DistributeBtcType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *DistributeBtcType) init(task *db.Task) error {
	var config Config
	err := go_format.FormatInstance.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.btcWallet = go_coin_btc.NewWallet(&chaincfg.MainNetParams)
	p.btcWallet.InitRpcClient(&go_coin_btc.RpcServerConfig{
		Url: config.BtcNodeUrl,
	})
	return nil
}

func (p *DistributeBtcType) doLoop(exitChan <-chan go_best_type.ExitType, task *db.Task) (constant.TaskStatusType, error) {
	err := p.init(task)
	if err != nil {
		return constant.TaskStatusType_ExitedWithErr, err
	}

	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			err := p.do(task)
			if err != nil {
				return constant.TaskStatusType_ExitedWithErr, err
			}
			if task.Interval != 0 {
				timer.Reset(time.Duration(task.Interval) * time.Second)
			} else {
				go p.BestTypeManager().ExitOne(p.Name(), go_best_type.ExitType_User)
			}
		case exitType := <-exitChan:
			switch exitType {
			case go_best_type.ExitType_System:
				p.Logger().InfoF("Exited by system.")
				return constant.TaskStatusType_WaitExec, nil
			case go_best_type.ExitType_User:
				p.Logger().InfoF("Exited by user.")
				return constant.TaskStatusType_Exited, nil
			}
		}
	}
}

func (p *DistributeBtcType) do(task *db.Task) error {
	wif, err := go_crypto.CryptoInstance.AesCbcDecrypt(global.GlobalConfig.Pass, p.config.Priv)
	if err != nil {
		return err
	}
	keyInfo, err := p.btcWallet.KeyInfoFromWif(wif)
	if err != nil {
		return err
	}
	addr, err := p.btcWallet.AddressFromPubKey(keyInfo.PubKey, go_coin_btc.ADDRESS_TYPE_P2TR)
	if err != nil {
		return err
	}

	addressDbs := make([]*db.BtcAddress, 0)
	err = go_mysql.MysqlInstance.RawSelect(
		&addressDbs,
		p.config.SelectAddressSql[0],
		p.config.SelectAddressSql[1],
	)
	if err != nil {
		return err
	}

	for _, addressDb := range addressDbs {
		err := util.CheckUnConfirmedCountAndWait(p.Logger(), task)
		if err != nil {
			return err
		}

		var btcAddressDb db.BtcAddress
		notFound, err := go_mysql.MysqlInstance.SelectById(
			&btcAddressDb,
			&go_mysql.SelectByIdParams{
				TableName: "btc_address",
				Select:    "*",
				Id:        addressDb.Id,
			},
		)
		if err != nil {
			return err
		}
		if notFound {
			continue
		}

		arr := strings.Split(p.config.Amount, "-")
		var targetAmount float64
		if len(arr) == 1 {
			targetAmount = go_format.FormatInstance.MustToFloat64(arr[0])
		} else {
			startAmount := go_format.FormatInstance.MustToFloat64(arr[0])
			endAmount := go_format.FormatInstance.MustToFloat64(arr[1])
			targetAmount = go_decimal.Decimal.MustStart(go_random.RandomInstance.MustRandomFloat64(startAmount, endAmount)).Round(8).MustEndForFloat64()
		}

		p.Logger().InfoF("Prepare send <%s> <%f> BTC.", btcAddressDb.Address, targetAmount)
		err = p.send(task, keyInfo.PrivKey, addr, btcAddressDb.Address, targetAmount)
		if err != nil {
			return err
		}
		p.Logger().InfoF("Distribute done. address id: %d, amount: %f", addressDb.Id, targetAmount)
	}

	return nil
}

func (p *DistributeBtcType) send(
	task *db.Task,
	priv string,
	fromAddress string,
	toAddr string,
	amount float64,
) error {
	feeRate, err := p.btcWallet.RpcClient.EstimateSmartFee()
	if err != nil {
		return err
	}

	btcUtxos, err := util.SelectUtxos(fromAddress, amount, 0.001)
	if err != nil {
		return err
	}

	outPointWithPrivs := make([]*go_coin_btc.UTXOWithPriv, 0)
	for _, btcUtxo := range btcUtxos {
		outPointWithPrivs = append(outPointWithPrivs, &go_coin_btc.UTXOWithPriv{
			Utxo: go_coin_btc.UTXO{
				TxId:  btcUtxo.TxId,
				Index: btcUtxo.Index,
			},
			Priv: priv,
		})
	}
	if len(outPointWithPrivs) == 0 {
		return errors.New("Balance not enough. no utxo")
	}

	p.Logger().InfoF("Build tx...")
	msgTx, newUtxos, _, err := p.btcWallet.BuildTx(
		outPointWithPrivs,
		fromAddress,
		toAddr,
		amount,
		feeRate*1.1,
	)
	if err != nil {
		return err
	}

	// 发送交易
	p.Logger().InfoF("Send tx...")
	txId, err := p.btcWallet.RpcClient.SendMsgTx(msgTx)
	if err != nil {
		return err
	}

	// 保存交易记录
	p.Logger().InfoF("Save tx record...")
	txHex, _ := p.btcWallet.MsgTxToHex(msgTx)
	_, err = go_mysql.MysqlInstance.Insert(
		"btc_tx",
		db.BtcTx{
			TaskId:  task.Id,
			TxId:    txId,
			Confirm: 0,
			TxHex:   txHex,
		},
	)
	if err != nil {
		return err
	}

	// 更新 utxo
	p.Logger().InfoF("Update from address utxo...")
	fromAddrNewUtxos := make([]constant.UTXO, 0)
	for _, newUtxo := range newUtxos {
		if strings.EqualFold(newUtxo.Address, fromAddress) {
			fromAddrNewUtxos = append(fromAddrNewUtxos, constant.UTXO{
				TxId:  newUtxo.TxId,
				Index: newUtxo.Index,
				Value: newUtxo.Value,
			})
		}
	}
	err = util.UpdateUtxo(
		fromAddress,
		btcUtxos,
		fromAddrNewUtxos,
	)
	if err != nil {
		return err
	}

	p.Logger().InfoF("Update to address utxo...")
	toAddrNewUtxos := make([]constant.UTXO, 0)
	for _, newUtxo := range newUtxos {
		if strings.EqualFold(newUtxo.Address, toAddr) {
			toAddrNewUtxos = append(toAddrNewUtxos, constant.UTXO{
				TxId:  newUtxo.TxId,
				Index: newUtxo.Index,
				Value: newUtxo.Value,
			})
		}
	}
	err = util.UpdateUtxo(
		toAddr,
		nil,
		toAddrNewUtxos,
	)
	if err != nil {
		return err
	}

	return nil
}
