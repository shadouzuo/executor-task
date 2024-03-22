package gather_btc

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/pkg/errors"

	go_best_type "github.com/pefish/go-best-type"
	go_coin_btc "github.com/pefish/go-coin-btc"
	go_crypto "github.com/pefish/go-crypto"
	go_format "github.com/pefish/go-format"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
	"github.com/shadouzuo/executor-task/pkg/util"
)

type GatherBtcType struct {
	go_best_type.BaseBestType
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type Config struct {
	Mnemonic         string   `json:"mnemonic"`
	Pass             string   `json:"pass"`
	SelectAddressSql []string `json:"select_address_sql"`
	TargetAddressId  uint64   `json:"target_address_id"`
	BtcNodeUrl       string   `json:"btc_node_url"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(name string) *GatherBtcType {
	t := &GatherBtcType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *GatherBtcType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
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

func (p *GatherBtcType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *GatherBtcType) init(task *constant.Task) error {
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

func (p *GatherBtcType) do(task *constant.Task) error {

	addresses := make([]*constant.BtcAddress, 0)
	err := go_mysql.MysqlInstance.RawSelect(
		&addresses,
		p.config.SelectAddressSql[0],
		p.config.SelectAddressSql[1],
	)
	if err != nil {
		return err
	}

	for _, addrDb := range addresses {
		err := util.CheckUnConfirmedCountAndWait(p.Logger(), task)
		if err != nil {
			return err
		}

		var targetAddrDb constant.BtcAddress
		notFound, err := go_mysql.MysqlInstance.SelectById(
			&targetAddrDb,
			&go_mysql.SelectByIdParams{
				TableName: "btc_address",
				Select:    "*",
				Id:        p.config.TargetAddressId,
			},
		)
		if err != nil {
			return err
		}
		if notFound {
			return errors.New("Target address not found.")
		}

		err = p.gatherBtc(task, addrDb, &targetAddrDb)
		if err != nil {
			return err
		}
		p.Logger().InfoF("Address id <%d> gather done.", addrDb.Id)
	}

	return nil
}

func (p *GatherBtcType) gatherBtc(
	task *constant.Task,
	fromAddrDb *constant.BtcAddress,
	toAddrDb *constant.BtcAddress,
) error {
	fromAddressUtxos := make([]constant.UTXO, 0)
	if fromAddrDb.Utxos == nil {
		p.Logger().Error("No utxos.")
		return nil
	}
	err := json.Unmarshal([]byte(*fromAddrDb.Utxos), &fromAddressUtxos)
	if err != nil {
		return err
	}
	if len(fromAddressUtxos) == 0 {
		p.Logger().Error("No utxos.")
		return nil
	}

	feeRate, err := p.btcWallet.RpcClient.EstimateSmartFee()
	if err != nil {
		return err
	}

	seedPass, err := go_crypto.CryptoInstance.AesCbcDecrypt(global.GlobalConfig.Pass, p.config.Pass)
	if err != nil {
		return err
	}
	seedHex := p.btcWallet.SeedHexByMnemonic(p.config.Mnemonic, seedPass)
	keyInfo, err := p.btcWallet.DeriveBySeedPath(seedHex, fmt.Sprintf("m/86'/0'/0'/0/%d", fromAddrDb.Index))
	if err != nil {
		return err
	}

	outPointWithPrivs := make([]*go_coin_btc.UTXOWithPriv, 0)
	for _, utxo := range fromAddressUtxos {
		outPointWithPrivs = append(outPointWithPrivs, &go_coin_btc.UTXOWithPriv{
			Utxo: go_coin_btc.UTXO{
				TxId:  utxo.TxId,
				Index: utxo.Index,
			},
			Priv: keyInfo.PrivKey,
		})
	}
	if len(outPointWithPrivs) == 0 {
		return errors.New("Balance not enough. no utxo")
	}

	p.Logger().InfoF("Build tx...")
	msgTx, newUtxos, _, err := p.btcWallet.BuildTx(
		outPointWithPrivs,
		"",
		toAddrDb.Address,
		0,
		feeRate*1.2,
	)
	if err != nil {
		return err
	}
	for _, utxo := range newUtxos {
		p.Logger().InfoF("tx_id: %s, addr: %s, value: %f, index: %d", utxo.TxId, utxo.Address, utxo.Value, utxo.Index)
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
		constant.BtcTx{
			TaskId:  task.Id,
			TxId:    txId,
			TxHex:   txHex,
			Confirm: 0,
		},
	)
	if err != nil {
		return err
	}

	// 更新 utxo
	p.Logger().InfoF("Update utxo...")
	_, err = go_mysql.MysqlInstance.Update(
		&go_mysql.UpdateParams{
			TableName: "btc_address",
			Update: map[string]interface{}{
				"utxos": "[]",
			},
			Where: map[string]interface{}{
				"address": fromAddrDb.Address,
			},
		},
	)
	if err != nil {
		return err
	}

	toAddressUtxos := make([]constant.UTXO, 0)
	if toAddrDb.Utxos != nil {
		err := json.Unmarshal([]byte(*toAddrDb.Utxos), &toAddressUtxos)
		if err != nil {
			return err
		}
	}
	for _, newUtxo := range newUtxos {
		if strings.EqualFold(newUtxo.Address, toAddrDb.Address) {
			toAddressUtxos = append(toAddressUtxos, constant.UTXO{
				TxId:  newUtxo.TxId,
				Index: newUtxo.Index,
				Value: newUtxo.Value,
			})
		}
	}

	b, _ := json.Marshal(toAddressUtxos)
	_, err = go_mysql.MysqlInstance.Update(
		&go_mysql.UpdateParams{
			TableName: "btc_address",
			Update: map[string]interface{}{
				"utxos": string(b),
			},
			Where: map[string]interface{}{
				"address": toAddrDb.Address,
			},
		},
	)
	if err != nil {
		return err
	}

	return nil
}
