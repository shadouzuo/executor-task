package print_tg_group_id

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/btcsuite/btcd/chaincfg"

	go_best_type "github.com/pefish/go-best-type"
	go_coin_btc "github.com/pefish/go-coin-btc"
	go_format "github.com/pefish/go-format"
	go_http "github.com/pefish/go-http"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
)

type PrintTgGroupIdType struct {
	go_best_type.BaseBestType
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type Config struct {
	BotToken  string `json:"bot_token"`
	GroupName string `json:"group_name"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(name string) *PrintTgGroupIdType {
	t := &PrintTgGroupIdType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *PrintTgGroupIdType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
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

func (p *PrintTgGroupIdType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *PrintTgGroupIdType) init(task *constant.Task) error {
	var config Config
	err := go_format.FormatInstance.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.btcWallet = go_coin_btc.NewWallet(&chaincfg.MainNetParams)
	return nil
}

func (p *PrintTgGroupIdType) do(task *constant.Task) error {
	var httpResult struct {
		Ok     bool `json:"ok"`
		Result []struct {
			Message struct {
				Chat struct {
					Id    int64  `json:"id"`
					Title string `json:"title"`
				} `json:"chat"`
			} `json:"message"`
		} `json:"result"`
	}
	_, _, err := go_http.NewHttpRequester(
		go_http.WithLogger(p.Logger()),
		go_http.WithTimeout(5*time.Second),
	).GetForStruct(
		go_http.RequestParam{
			Url: fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", p.config.BotToken),
		},
		&httpResult,
	)
	if err != nil {
		return err
	}
	groupId := 0
	for _, chat := range httpResult.Result {
		if chat.Message.Chat.Title == p.config.GroupName {
			groupId = int(chat.Message.Chat.Id)
			break
		}
	}
	if groupId == 0 {
		return errors.New("Group id not found.")
	}

	_, err = go_mysql.MysqlInstance.Update(
		&go_mysql.UpdateParams{
			TableName: "task",
			Update: map[string]interface{}{
				"mark": groupId,
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
