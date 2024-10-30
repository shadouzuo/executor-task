package print_tg_group_id

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/btcsuite/btcd/chaincfg"

	go_coin_btc "github.com/pefish/go-coin-btc"
	go_format "github.com/pefish/go-format"
	go_http "github.com/pefish/go-http"
	i_logger "github.com/pefish/go-interface/i-logger"
	"github.com/shadouzuo/executor-task/pkg/constant"
)

type PrintTgGroupIdType struct {
	logger    i_logger.ILogger
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

func New(logger i_logger.ILogger) *PrintTgGroupIdType {
	t := &PrintTgGroupIdType{
		logger: logger,
	}
	return t
}

func (p *PrintTgGroupIdType) Start(ctx context.Context, task *constant.Task) (any, error) {
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

func (p *PrintTgGroupIdType) init(task *constant.Task) error {
	var config Config
	err := go_format.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.btcWallet = go_coin_btc.NewWallet(&chaincfg.MainNetParams, p.logger)
	return nil
}

func (p *PrintTgGroupIdType) do(task *constant.Task) (interface{}, error) {
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
		go_http.WithLogger(p.logger),
		go_http.WithTimeout(5*time.Second),
	).GetForStruct(
		&go_http.RequestParams{
			Url: fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates", p.config.BotToken),
		},
		&httpResult,
	)
	if err != nil {
		return "", err
	}
	groupId := 0
	for i := len(httpResult.Result) - 1; i >= 0; i-- {
		chat := httpResult.Result[i]
		if chat.Message.Chat.Title == p.config.GroupName {
			groupId = int(chat.Message.Chat.Id)
			break
		}
	}
	if groupId == 0 {
		return "", errors.New("Group id not found.")
	}

	return groupId, nil
}
