package transfer_btc

import (
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/btcsuite/btcd/chaincfg"

	go_best_type "github.com/pefish/go-best-type"
	go_coin_btc "github.com/pefish/go-coin-btc"
	go_crypto "github.com/pefish/go-crypto"
	go_format "github.com/pefish/go-format"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
	"github.com/shadouzuo/executor-task/pkg/util"
)

type TransferBtcType struct {
	go_best_type.BaseBestType
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type AddressInfo struct {
	Address string  `json:"address"`
	Amount  float64 `json:"amount"`
}

type Config struct {
	Wif          string        `json:"wif"`
	AddressInfos []AddressInfo `json:"address_infos"`
	BtcNodeUrl   string        `json:"btc_node_url"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(name string) *TransferBtcType {
	t := &TransferBtcType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *TransferBtcType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
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

func (p *TransferBtcType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *TransferBtcType) init(task *constant.Task) error {
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

func (p *TransferBtcType) do(task *constant.Task) error {
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

	for _, addressInfo := range p.config.AddressInfos {
		err := util.CheckUnConfirmedCountAndWait(p.Logger(), task)
		if err != nil {
			return err
		}

		p.Logger().InfoF("Prepare send <%s> <%f> BTC.", addressInfo.Address, addressInfo.Amount)
		_, _, spentUtxos, newUtxos, err := util.SendBtc(
			p.Logger(),
			p.btcWallet,
			keyInfo.PrivKey,
			fromAddress,
			addressInfo.Address,
			addressInfo.Amount,
		)
		if err != nil {
			return err
		}
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
		p.Logger().InfoF("Transfer done. address: %s, amount: %f", addressInfo.Address, addressInfo.Amount)
	}

	return nil
}
