package watch_contract_read_result

import (
	"context"
	"fmt"
	"math/big"
	"time"

	go_coin_eth "github.com/pefish/go-coin-eth"
	go_decimal "github.com/pefish/go-decimal"
	go_format "github.com/pefish/go-format"
	i_logger "github.com/pefish/go-interface/i-logger"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/util"
)

type WatchContractReadResultType struct {
	logger    i_logger.ILogger
	config    *Config
	ethWallet *go_coin_eth.Wallet
}

type Config struct {
	Method         string   `json:"method"`
	Params         []string `json:"params"`
	Contract       string   `json:"contract"`
	Abi            string   `json:"abi"`
	NodeUrl        string   `json:"node_url"`
	AlertCondition struct {
		Lt int `json:"lt"`
	} `json:"alert_condition"`
	WeiXinAlertKey string `json:"weixin_alert_key"`
}

func New(logger i_logger.ILogger) *WatchContractReadResultType {
	t := &WatchContractReadResultType{
		logger: logger,
	}
	return t
}

func (p *WatchContractReadResultType) Start(ctx context.Context, task *constant.Task) (any, error) {
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
			p.logger.InfoF("任务 <%s> 结束", task.Name)
			return nil, nil
		}
	}
}

func (p *WatchContractReadResultType) init(task *constant.Task) error {
	var config Config
	err := go_format.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	w, err := go_coin_eth.NewWallet(p.logger).InitRemote(&go_coin_eth.UrlParam{
		RpcUrl: config.NodeUrl,
	})
	if err != nil {
		return err
	}
	p.ethWallet = w
	return nil
}

func (p *WatchContractReadResultType) do(task *constant.Task) error {

	var result *big.Int
	err := p.ethWallet.CallContractConstant(
		&result,
		p.config.Contract,
		p.config.Abi,
		p.config.Method,
		nil,
		nil,
	)
	if err != nil {
		return err
	}

	p.logger.InfoF("result: %s", result.String())

	if go_decimal.Decimal.MustStart(result).MustLt(p.config.AlertCondition.Lt) {
		util.Alert(
			p.logger,
			fmt.Sprintf("Venus 可以升级为主要用户了。%s", result.String()),
			p.config.WeiXinAlertKey,
		)
	}

	return nil
}
