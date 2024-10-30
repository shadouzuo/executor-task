package merlin_print_aa_address

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/btcsuite/btcd/chaincfg"

	go_coin_btc "github.com/pefish/go-coin-btc"
	go_format "github.com/pefish/go-format"
	go_http "github.com/pefish/go-http"
	i_logger "github.com/pefish/go-interface/i-logger"
	t_mysql "github.com/pefish/go-interface/t-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
)

type MerlinPrintAaAddressType struct {
	logger    i_logger.ILogger
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type Config struct {
	SelectAddressSql []string `json:"select_address_sql"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(logger i_logger.ILogger) *MerlinPrintAaAddressType {
	t := &MerlinPrintAaAddressType{
		logger: logger,
	}
	return t
}

func (p *MerlinPrintAaAddressType) Start(ctx context.Context, task *constant.Task) (any, error) {
	err := p.init(task)
	if err != nil {
		return nil, err
	}

	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			result, err := p.do(task)
			if err != nil {
				return nil, err
			}
			if task.Interval != 0 {
				timer.Reset(time.Duration(task.Interval) * time.Second)
				continue
			}
			return result, nil
		case <-ctx.Done():
			return nil, nil
		}
	}
}

func (p *MerlinPrintAaAddressType) init(task *constant.Task) error {
	var config Config
	err := go_format.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.btcWallet = go_coin_btc.NewWallet(&chaincfg.MainNetParams, p.logger)
	return nil
}

func (p *MerlinPrintAaAddressType) do(task *constant.Task) (interface{}, error) {

	addresses := make([]*constant.BtcAddress, 0)
	err := global.MysqlInstance.RawSelect(
		&addresses,
		p.config.SelectAddressSql[0],
		p.config.SelectAddressSql[1],
	)
	if err != nil {
		return "", err
	}

	aaAddresses := make([]string, 0)
	for _, addrDb := range addresses {
		aaAddress, err := p.getAaAddress(addrDb.Address)
		if err != nil {
			return nil, err
		}
		aaAddresses = append(aaAddresses, aaAddress)
		_, err = global.MysqlInstance.Update(&t_mysql.UpdateParams{
			TableName: "btc_address",
			Update: map[string]interface{}{
				"merlin_address": aaAddress,
			},
			Where: map[string]interface{}{
				"id": addrDb.Id,
			},
		})
		if err != nil {
			return nil, err
		}
		p.logger.InfoF("<%d> done.", addrDb.Id)
		time.Sleep(time.Second)
	}

	return strings.Join(aaAddresses, ","), nil
}

func (p *MerlinPrintAaAddressType) getAaAddress(btcAddress string) (string, error) {
	var httpResult struct {
		Code uint64 `json:"code"`
		Data struct {
			AaAddress string `json:"aa"`
		} `json:"data"`
		Msg string `json:"msg"`
	}
	_, _, err := go_http.NewHttpRequester(go_http.WithLogger(p.logger)).GetForStruct(
		&go_http.RequestParams{
			Url: "https://bridge.merlinchain.io/api/v1/address/match_by_btc",
			Params: map[string]interface{}{
				"address": btcAddress,
			},
		},
		&httpResult,
	)
	if err != nil {
		return "", err
	}
	if httpResult.Code != 0 {
		return "", errors.New(httpResult.Msg)
	}
	return httpResult.Data.AaAddress, nil
}
