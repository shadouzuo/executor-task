package update_btc_utxo

import (
	"encoding/json"
	"time"

	go_best_type "github.com/pefish/go-best-type"
	"github.com/pefish/go-coin-btc/pkg/btccom"
	go_decimal "github.com/pefish/go-decimal"
	go_format "github.com/pefish/go-format"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/db"
)

type UpdateBtcUtxoType struct {
	go_best_type.BaseBestType
	config       *Config
	btcComClient *btccom.BtcComClient
}

type Config struct {
	SelectAddressSql []string `json:"select_address_sql"`
}

type ActionTypeData struct {
	Task *db.Task
}

func New(name string) *UpdateBtcUtxoType {
	t := &UpdateBtcUtxoType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *UpdateBtcUtxoType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
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

func (p *UpdateBtcUtxoType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *UpdateBtcUtxoType) init(task *db.Task) error {
	var config Config
	err := go_format.FormatInstance.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.btcComClient = btccom.NewBtcComClient(
		p.Logger(),
		10*time.Second,
		"",
	)
	return nil
}

func (p *UpdateBtcUtxoType) doLoop(exitChan <-chan go_best_type.ExitType, task *db.Task) (constant.TaskStatusType, error) {
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

func (p *UpdateBtcUtxoType) do(task *db.Task) error {
	addresses := make([]*db.BtcAddress, 0)
	err := go_mysql.MysqlInstance.RawSelect(
		&addresses,
		p.config.SelectAddressSql[0],
		p.config.SelectAddressSql[1],
	)
	if err != nil {
		return err
	}
	for _, addrDb := range addresses {
		err := p.updateUtxo(addrDb)
		if err != nil {
			return err
		}
		p.Logger().InfoF("Utxo of <%s> updated.", addrDb.Address)
	}

	return nil
}

func (p *UpdateBtcUtxoType) updateUtxo(addrDb *db.BtcAddress) error {
	results, err := p.btcComClient.ListUnspent(addrDb.Address)
	if err != nil {
		return err
	}

	utxos := make([]constant.UTXO, 0)
	for _, result := range results {
		utxos = append(utxos, constant.UTXO{
			TxId:  result.TxHash,
			Index: result.TxOutputN,
			Value: go_decimal.Decimal.MustStart(result.Value).MustUnShiftedBy(8).MustEndForFloat64(),
		})
	}

	b, _ := json.Marshal(utxos)

	_, err = go_mysql.MysqlInstance.Update(&go_mysql.UpdateParams{
		TableName: "btc_address",
		Update: map[string]interface{}{
			"utxos": string(b),
		},
		Where: map[string]interface{}{
			"address": addrDb.Address,
		},
	})
	if err != nil {
		return err
	}

	return nil
}
