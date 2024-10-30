package update_btc_confirm

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

type UpdateBtcConfirmType struct {
	logger    i_logger.ILogger
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type Config struct {
	BtcNodeUrl string `json:"btc_node_url"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(logger i_logger.ILogger) *UpdateBtcConfirmType {
	t := &UpdateBtcConfirmType{
		logger: logger,
	}
	return t
}

func (p *UpdateBtcConfirmType) Start(ctx context.Context, task *constant.Task) (any, error) {
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

func (p *UpdateBtcConfirmType) init(task *constant.Task) error {
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

func (p *UpdateBtcConfirmType) do(task *constant.Task) error {
	btcTxs := make([]constant.BtcTx, 0)
	err := global.MysqlInstance.Select(
		&btcTxs,
		&t_mysql.SelectParams{
			TableName: "btc_tx",
			Select:    "*",
			Where: map[string]interface{}{
				"confirm": 0,
			},
		},
	)
	if err != nil {
		return err
	}

	if len(btcTxs) == 0 {
		p.logger.InfoF("Nothing.")
	}

	for _, tx := range btcTxs {
		err := p.processTxId(&tx)
		if err != nil {
			p.logger.Error(err)
			continue
		}
		time.Sleep(time.Second)
	}

	return nil
}

func (p *UpdateBtcConfirmType) processTxId(record *constant.BtcTx) error {
	toTxInfo, err := p.btcWallet.RpcClient.GetRawTransaction(record.TxId)
	if err != nil {
		return err
	}
	if toTxInfo.Confirmations == 0 {
		p.logger.DebugF("Confirmations not update. tx id: %s", record.TxId)
		return nil
	}
	// update
	_, err = global.MysqlInstance.Update(
		&t_mysql.UpdateParams{
			TableName: "btc_tx",
			Update: map[string]interface{}{
				"confirm": toTxInfo.Confirmations,
			},
			Where: map[string]interface{}{
				"id": record.Id,
			},
		},
	)
	if err != nil {
		return err
	}
	p.logger.InfoF("Processed. tx_id: %s, confirmations: %d", record.TxId, toTxInfo.Confirmations)

	return nil
}
