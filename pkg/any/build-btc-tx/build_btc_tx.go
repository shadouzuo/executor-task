package build_btc_tx

import (
	"errors"
	"time"

	"github.com/btcsuite/btcd/chaincfg"

	go_best_type "github.com/pefish/go-best-type"
	go_coin_btc "github.com/pefish/go-coin-btc"
	go_format "github.com/pefish/go-format"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
)

type BuildBtcTxType struct {
	go_best_type.BaseBestType
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type Config struct {
	BtcNodeUrl     string             `json:"btc_node_url"`
	UTXOs          []go_coin_btc.UTXO `json:"utxos"`
	Wif            string             `json:"wif"`
	ChangeAddress  string             `json:"change_address"`
	TargetAddress  string             `json:"target_address"`
	TargetValueBtc float64            `json:"target_value_btc"`
	FeeRate        float64            `json:"fee_rate"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(name string) *BuildBtcTxType {
	t := &BuildBtcTxType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *BuildBtcTxType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
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
				p.BestTypeManager().ExitSelf(p.Name())
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

func (p *BuildBtcTxType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *BuildBtcTxType) init(task *constant.Task) error {
	var config Config
	err := go_format.FormatInstance.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.btcWallet = go_coin_btc.NewWallet(&chaincfg.MainNetParams)
	p.btcWallet.InitRpcClient(&go_coin_btc.RpcServerConfig{
		Url: config.BtcNodeUrl,
	}, 10*time.Second)
	return nil
}

func (p *BuildBtcTxType) do(task *constant.Task) error {

	keyInfo, err := p.btcWallet.KeyInfoFromWif(p.config.Wif)
	if err != nil {
		return err
	}

	utxoWithPrivs := make([]*go_coin_btc.UTXOWithPriv, 0)
	for _, utxo := range p.config.UTXOs {
		utxoWithPrivs = append(utxoWithPrivs, &go_coin_btc.UTXOWithPriv{
			Utxo: utxo,
			Priv: keyInfo.PrivKey,
		})
	}
	msgTx, _, _, err := p.btcWallet.BuildTx(
		utxoWithPrivs,
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

	_, err = go_mysql.MysqlInstance.Update(
		&go_mysql.UpdateParams{
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
