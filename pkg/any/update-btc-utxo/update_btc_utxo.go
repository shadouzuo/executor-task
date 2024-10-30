package update_btc_utxo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pefish/go-coin-btc/btccom"
	go_decimal "github.com/pefish/go-decimal"
	go_format "github.com/pefish/go-format"
	i_logger "github.com/pefish/go-interface/i-logger"
	t_mysql "github.com/pefish/go-interface/t-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
)

type UpdateBtcUtxoType struct {
	logger       i_logger.ILogger
	config       *Config
	btcComClient *btccom.BtcComClient
}

type Config struct {
	SelectAddressSql []string `json:"select_address_sql"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(logger i_logger.ILogger) *UpdateBtcUtxoType {
	t := &UpdateBtcUtxoType{
		logger: logger,
	}
	return t
}

func (p *UpdateBtcUtxoType) Start(ctx context.Context, task *constant.Task) (any, error) {
	err := p.init(task)
	if err != nil {
		return nil, err
	}

	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			err := p.do(task)
			if err != nil {
				return nil, err
			}
			if task.Interval != 0 {
				timer.Reset(time.Duration(task.Interval) * time.Second)
				continue
			}
			return "", nil
		case <-ctx.Done():
			return nil, nil
		}
	}
}

func (p *UpdateBtcUtxoType) init(task *constant.Task) error {
	var config Config
	err := go_format.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.btcComClient = btccom.NewBtcComClient(
		p.logger,
		20*time.Second,
		"",
	)
	return nil
}

func (p *UpdateBtcUtxoType) do(task *constant.Task) error {
	addresses := make([]*constant.BtcAddress, 0)
	err := global.MysqlInstance.RawSelect(
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
		p.logger.InfoF("Utxo of index <%d> updated.", addrDb.Index)
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
	_, err = global.MysqlInstance.Update(&t_mysql.UpdateParams{
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
