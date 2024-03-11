package any

import (
	"fmt"
	"time"

	"github.com/btcsuite/btcd/chaincfg"

	go_best_type "github.com/pefish/go-best-type"
	go_coin_btc "github.com/pefish/go-coin-btc"
	go_crypto "github.com/pefish/go-crypto"
	go_format "github.com/pefish/go-format"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/db"
	"github.com/shadouzuo/executor-task/pkg/global"
)

type GeneBtcAddressType struct {
	go_best_type.BaseBestType
}

type GeneBtcAddressConfig struct {
	Mnemonic   string `json:"mnemonic"`
	Pass       string `json:"pass"`
	StartIndex uint64 `json:"start_index"`
	EndIndex   uint64 `json:"end_index"`
	TaskId     uint64 `json:"task_id"`
}

func NewGeneBtcAddressType(name string) *GeneBtcAddressType {
	t := &GeneBtcAddressType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *GeneBtcAddressType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
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

func (p *GeneBtcAddressType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *GeneBtcAddressType) doLoop(exitChan <-chan go_best_type.ExitType, task *db.Task) (constant.TaskStatusType, error) {
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

func (p *GeneBtcAddressType) do(task *db.Task) error {
	var config GeneBtcAddressConfig
	err := go_format.FormatInstance.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}

	addresses := make([]db.BtcAddress, 0)

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
		addresses = append(addresses, db.BtcAddress{
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
