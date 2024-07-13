package distribute_btc

import (
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/btcsuite/btcd/chaincfg"

	go_best_type "github.com/pefish/go-best-type"
	go_coin_btc "github.com/pefish/go-coin-btc"
	go_crypto "github.com/pefish/go-crypto"
	go_decimal "github.com/pefish/go-decimal"
	go_format "github.com/pefish/go-format"
	go_mysql "github.com/pefish/go-mysql"
	go_random "github.com/pefish/go-random"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
	"github.com/shadouzuo/executor-task/pkg/util"
)

type DistributeBtcType struct {
	go_best_type.BaseBestType
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type Config struct {
	Wif              string   `json:"wif"`
	Amount           string   `json:"amount"`
	SelectAddressSql []string `json:"select_address_sql"`
	BtcNodeUrl       string   `json:"btc_node_url"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(name string) *DistributeBtcType {
	t := &DistributeBtcType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *DistributeBtcType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	task := ask.Data.(ActionTypeData).Task

	err := p.init(task)
	if err != nil {
		ask.AnswerChan <- constant.TaskResult{
			BestType: p,
			Task:     task,
			Data:     "",
			Err:      err,
		}
		return nil
	}

	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			err := p.do(task)
			if err != nil {
				ask.AnswerChan <- constant.TaskResult{
					BestType: p,
					Task:     task,
					Data:     "",
					Err:      err,
				}
				p.BestTypeManager().ExitSelf(p.Name())
				return nil
			}
			if task.Interval != 0 {
				timer.Reset(time.Duration(task.Interval) * time.Second)
				continue
			}
			ask.AnswerChan <- constant.TaskResult{
				BestType: p,
				Task:     task,
				Data:     "result",
				Err:      nil,
			}
			p.BestTypeManager().ExitSelf(p.Name())
			return nil
		case exitType := <-exitChan:
			switch exitType {
			case go_best_type.ExitType_System:
				ask.AnswerChan <- constant.TaskResult{
					BestType: p,
					Task:     task,
					Data:     "",
					Err:      errors.New("Exited by system."),
				}
				return nil
			case go_best_type.ExitType_User:
				ask.AnswerChan <- constant.TaskResult{
					BestType: p,
					Task:     task,
					Data:     "",
					Err:      errors.New("Exited by user."),
				}
				return nil
			}
		}
	}
}

func (p *DistributeBtcType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *DistributeBtcType) init(task *constant.Task) error {
	var config Config
	err := go_format.FormatInstance.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.btcWallet = go_coin_btc.NewWallet(&chaincfg.MainNetParams)
	p.btcWallet.InitRpcClient(&go_coin_btc.RpcServerConfig{
		Url: config.BtcNodeUrl,
	}, 10*time.Second)
	return nil
}

func (p *DistributeBtcType) do(task *constant.Task) error {
	wif, err := go_crypto.CryptoInstance.AesCbcDecrypt(global.GlobalConfig.Pass, p.config.Wif)
	if err != nil {
		return err
	}
	keyInfo, err := p.btcWallet.KeyInfoFromWif(wif)
	if err != nil {
		return err
	}
	fromAddress, err := p.btcWallet.AddressFromPubKey(keyInfo.PubKey, go_coin_btc.ADDRESS_TYPE_P2TR)
	if err != nil {
		return err
	}

	addressDbs := make([]*constant.BtcAddress, 0)
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

		var btcAddressDb constant.BtcAddress
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
		txId, msgTx, spentUtxos, newUtxos, err := util.SendBtc(
			p.Logger(),
			p.btcWallet,
			keyInfo.PrivKey,
			fromAddress,
			btcAddressDb.Address,
			targetAmount,
		)
		if err != nil {
			return err
		}

		// 保存交易记录
		p.Logger().InfoF("Save tx record...")
		txHex, _ := p.btcWallet.MsgTxToHex(msgTx)
		_, err = go_mysql.MysqlInstance.Insert(
			"btc_tx",
			constant.BtcTx{
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
			spentUtxos,
			fromAddrNewUtxos,
		)
		if err != nil {
			return err
		}

		p.Logger().InfoF("Update to address utxo...")
		toAddrNewUtxos := make([]constant.UTXO, 0)
		for _, newUtxo := range newUtxos {
			if strings.EqualFold(newUtxo.Address, btcAddressDb.Address) {
				toAddrNewUtxos = append(toAddrNewUtxos, constant.UTXO{
					TxId:  newUtxo.TxId,
					Index: newUtxo.Index,
					Value: newUtxo.Value,
				})
			}
		}
		err = util.UpdateUtxo(
			btcAddressDb.Address,
			nil,
			toAddrNewUtxos,
		)
		if err != nil {
			return err
		}
		p.Logger().InfoF("Distribute done. address id: %d, amount: %f", addressDb.Id, targetAmount)
	}

	return nil
}
