package check_mbtc_transfer

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

type MerlinPrintAaAddressType struct {
	go_best_type.BaseBestType
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type Config struct {
	SelectAddressSql []string `json:"select_address_sql"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(name string) *MerlinPrintAaAddressType {
	t := &MerlinPrintAaAddressType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *MerlinPrintAaAddressType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
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
				Data:     nil,
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

func (p *MerlinPrintAaAddressType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *MerlinPrintAaAddressType) init(task *constant.Task) error {
	var config Config
	err := go_format.FormatInstance.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.btcWallet = go_coin_btc.NewWallet(&chaincfg.MainNetParams)
	return nil
}

func (p *MerlinPrintAaAddressType) do(task *constant.Task) error {

	addresses := make([]*constant.BtcAddress, 0)
	err := go_mysql.MysqlInstance.RawSelect(
		&addresses,
		p.config.SelectAddressSql[0],
		p.config.SelectAddressSql[1],
	)
	if err != nil {
		return err
	}

	for _, addrDb := range addresses {
		aaAddress, err := p.getAaAddress(addrDb.Address)
		if err != nil {
			return err
		}

		var httpResult struct {
			Result struct {
				Data struct {
					Json struct {
						List []struct {
							Symbol    string `json:"symbol"`
							ToAddress string `json:"to_address"`
						} `json:"list"`
					} `json:"json"`
				} `json:"data"`
			} `json:"result"`
		}
		_, _, err = go_http.NewHttpRequester(go_http.WithTimeout(5*time.Second)).GetForStruct(
			&go_http.RequestParams{
				Url: "https://scan.merlinchain.io/api/trpc/address.getAddressTokenTxList",
				Params: map[string]interface{}{
					"input": fmt.Sprintf(`{"json":{"address":"%s","tokenType":"erc20","take":20,"desc":null,"cursor":null},"meta":{"values":{"desc":["undefined"],"cursor":["undefined"]}}}`, aaAddress),
				},
			},
			&httpResult,
		)
		if err != nil {
			return err
		}
		if len(httpResult.Result.Data.Json.List) > 0 && httpResult.Result.Data.Json.List[0].Symbol == "M-BTC" &&
			httpResult.Result.Data.Json.List[0].ToAddress == "0x34faee07ac991a2e2f3bf7cf799e3d30c839ff44" {
			p.Logger().InfoF("<%d> is ok.", addrDb.Id)
			continue
		}
		p.Logger().InfoF("<%d> wrong.", addrDb.Id)
		time.Sleep(time.Second)
	}

	return nil
}

func (p *MerlinPrintAaAddressType) getAaAddress(btcAddress string) (string, error) {
	var httpResult struct {
		Code uint64 `json:"code"`
		Data struct {
			AaAddress string `json:"aa"`
		} `json:"data"`
		Msg string `json:"msg"`
	}
	_, _, err := go_http.NewHttpRequester(go_http.WithLogger(p.Logger())).GetForStruct(
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
