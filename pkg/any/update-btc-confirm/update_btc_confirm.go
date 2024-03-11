package update_btc_confirm

import (
	"time"

	"github.com/btcsuite/btcd/chaincfg"

	go_best_type "github.com/pefish/go-best-type"
	go_coin_btc "github.com/pefish/go-coin-btc"
	go_format "github.com/pefish/go-format"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/db"
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
	Task *db.Task
}

func New(name string) *UpdateBtcConfirmType {
	t := &UpdateBtcConfirmType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *UpdateBtcConfirmType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
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

func (p *UpdateBtcConfirmType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *UpdateBtcConfirmType) init(task *db.Task) error {
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

func (p *UpdateBtcConfirmType) doLoop(exitChan <-chan go_best_type.ExitType, task *db.Task) (constant.TaskStatusType, error) {
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
			p.Logger().InfoF("Loop done.")
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

func (p *UpdateBtcConfirmType) do(task *db.Task) error {
	btcTxs := make([]db.BtcTx, 0)
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
	}

	return nil
}

func (p *UpdateBtcConfirmType) processTxId(record *db.BtcTx) error {
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
