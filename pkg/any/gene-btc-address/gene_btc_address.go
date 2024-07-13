package gene_btc_address

import (
	"errors"
	"fmt"
	"time"

	"github.com/btcsuite/btcd/chaincfg"

	go_best_type "github.com/pefish/go-best-type"
	go_coin_btc "github.com/pefish/go-coin-btc"
	go_crypto "github.com/pefish/go-crypto"
	go_format "github.com/pefish/go-format"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
)

type GeneBtcAddressType struct {
	go_best_type.BaseBestType
}

type ActionTypeData struct {
	Task *constant.Task
}

type GeneBtcAddressConfig struct {
	Mnemonic   string `json:"mnemonic"`
	Pass       string `json:"pass"`
	StartIndex uint64 `json:"start_index"`
	EndIndex   uint64 `json:"end_index"`
	TaskId     uint64 `json:"task_id"`
}

func New(name string) *GeneBtcAddressType {
	t := &GeneBtcAddressType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *GeneBtcAddressType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	task := ask.Data.(ActionTypeData).Task

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

func (p *GeneBtcAddressType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *GeneBtcAddressType) do(task *constant.Task) error {
	var config GeneBtcAddressConfig
	err := go_format.FormatInstance.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}

	addresses := make([]constant.BtcAddress, 0)

	wallet := go_coin_btc.NewWallet(&chaincfg.MainNetParams)
	seedPass, err := go_crypto.CryptoInstance.AesCbcDecrypt(global.GlobalConfig.Pass, config.Pass)
	if err != nil {
		return err
	}
	seedHex := wallet.SeedHexByMnemonic(config.Mnemonic, seedPass)
	for i := config.StartIndex; i < config.EndIndex; i++ {
		result, err := wallet.DeriveBySeedPath(seedHex, fmt.Sprintf("m/86'/0'/0'/0/%d", i))
		if err != nil {
			return err
		}
		taprootAddr, err := wallet.AddressFromPubKey(result.PubKey, go_coin_btc.ADDRESS_TYPE_P2TR)
		if err != nil {
			return err
		}
		addresses = append(addresses, constant.BtcAddress{
			Address: taprootAddr,
			Index:   i,
		})
	}

	_, err = go_mysql.MysqlInstance.Insert("btc_address", addresses)
	if err != nil {
		return err
	}

	return nil
}
