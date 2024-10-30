package build_btc_tx

import (
	"context"
	"time"

	"github.com/btcsuite/btcd/chaincfg"

	go_coin_btc "github.com/pefish/go-coin-btc"
	go_format "github.com/pefish/go-format"
	i_logger "github.com/pefish/go-interface/i-logger"
	t_mysql "github.com/pefish/go-interface/t-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
)

type BuildBtcTxType struct {
	logger    i_logger.ILogger
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type Config struct {
	BtcNodeUrl     string                  `json:"btc_node_url"`
	UTXOs          []*go_coin_btc.OutPoint `json:"utxos"`
	Wif            string                  `json:"wif"`
	ChangeAddress  string                  `json:"change_address"`
	TargetAddress  string                  `json:"target_address"`
	TargetValueBtc float64                 `json:"target_value_btc"`
	FeeRate        int64                   `json:"fee_rate"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(logger i_logger.ILogger) *BuildBtcTxType {
	t := &BuildBtcTxType{
		logger: logger,
	}
	return t
}

func (p *BuildBtcTxType) Start(ctx context.Context, task *constant.Task) (any, error) {
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

func (p *BuildBtcTxType) init(task *constant.Task) error {
	var config Config
	err := go_format.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.btcWallet = go_coin_btc.NewWallet(&chaincfg.MainNetParams, p.logger)
	p.btcWallet.InitRpcClient(&go_coin_btc.RpcServerConfig{
		Url: config.BtcNodeUrl,
	}, 10*time.Second)
	return nil
}

func (p *BuildBtcTxType) do(task *constant.Task) error {

	err := p.btcWallet.AddAccount(p.config.Wif)
	if err != nil {
		return err
	}

	msgTx, _, _, err := p.btcWallet.BuildTx(
		p.config.UTXOs,
		p.config.ChangeAddress,
		p.config.TargetAddress,
		p.config.TargetValueBtc,
		p.config.FeeRate,
	)
	if err != nil {
		return err
	}
	txHex, err := p.btcWallet.MsgTxToHex(msgTx)
	if err != nil {
		return err
	}

	_, err = global.MysqlInstance.Update(
		&t_mysql.UpdateParams{
			TableName: "task",
			Update: map[string]interface{}{
				"mark": txHex,
			},
			Where: map[string]interface{}{
				"id": task.Id,
			},
		},
	)
	if err != nil {
		return err
	}

	return nil
}
