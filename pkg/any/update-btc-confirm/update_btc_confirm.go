package update_btc_confirm

import (
	"time"

	"github.com/pkg/errors"

	"github.com/btcsuite/btcd/chaincfg"

	go_best_type "github.com/pefish/go-best-type"
	go_coin_btc "github.com/pefish/go-coin-btc"
	go_format "github.com/pefish/go-format"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
)

type UpdateBtcConfirmType struct {
	go_best_type.BaseBestType
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type Config struct {
	BtcNodeUrl string `json:"btc_node_url"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(name string) *UpdateBtcConfirmType {
	t := &UpdateBtcConfirmType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *UpdateBtcConfirmType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
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

func (p *UpdateBtcConfirmType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *UpdateBtcConfirmType) init(task *constant.Task) error {
	var config Config
	err := go_format.FormatInstance.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.btcWallet = go_coin_btc.NewWallet(&chaincfg.MainNetParams)
	p.btcWallet.InitRpcClient(&go_coin_btc.RpcServerConfig{
		Url: config.BtcNodeUrl,
	})
	return nil
}

func (p *UpdateBtcConfirmType) do(task *constant.Task) error {
	btcTxs := make([]constant.BtcTx, 0)
	err := go_mysql.MysqlInstance.Select(
		&btcTxs,
		&go_mysql.SelectParams{
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
		p.Logger().InfoF("Nothing.")
	}

	for _, tx := range btcTxs {
		err := p.processTxId(&tx)
		if err != nil {
			p.Logger().Error(err)
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
		p.Logger().DebugF("Confirmations not update. tx id: %s", record.TxId)
		return nil
	}
	// update
	_, err = go_mysql.MysqlInstance.Update(
		&go_mysql.UpdateParams{
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
	p.Logger().InfoF("Processed. tx_id: %s, confirmations: %d", record.TxId, toTxInfo.Confirmations)

	return nil
}
