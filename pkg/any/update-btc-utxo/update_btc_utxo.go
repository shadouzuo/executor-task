package update_btc_utxo

import (
	"encoding/json"
	"time"

	"github.com/pkg/errors"

	go_best_type "github.com/pefish/go-best-type"
	"github.com/pefish/go-coin-btc/pkg/btccom"
	go_decimal "github.com/pefish/go-decimal"
	go_format "github.com/pefish/go-format"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
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
	Task *constant.Task
}

func New(name string) *UpdateBtcUtxoType {
	t := &UpdateBtcUtxoType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *UpdateBtcUtxoType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
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

func (p *UpdateBtcUtxoType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *UpdateBtcUtxoType) init(task *constant.Task) error {
	var config Config
	err := go_format.FormatInstance.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.btcComClient = btccom.NewBtcComClient(
		p.Logger(),
		20*time.Second,
		"",
	)
	return nil
}

func (p *UpdateBtcUtxoType) do(task *constant.Task) error {
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
		err := p.updateUtxo(addrDb)
		if err != nil {
			return err
		}
		p.Logger().InfoF("Utxo of <%s> updated.", addrDb.Address)
		time.Sleep(3 * time.Second)
	}

	return nil
}

func (p *UpdateBtcUtxoType) updateUtxo(addrDb *constant.BtcAddress) error {
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
	if err != nil && err.Error() != "No affected rows." {
		return err
	}

	return nil
}
