package print_wifs

import (
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/btcsuite/btcd/chaincfg"

	go_best_type "github.com/pefish/go-best-type"
	go_coin_btc "github.com/pefish/go-coin-btc"
	go_crypto "github.com/pefish/go-crypto"
	go_format "github.com/pefish/go-format"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
)

type PrintWifsType struct {
	go_best_type.BaseBestType
	config    *Config
	btcWallet *go_coin_btc.Wallet
}

type Config struct {
	Mnemonic         string   `json:"mnemonic"`
	Pass             string   `json:"pass"`
	SelectAddressSql []string `json:"select_address_sql"`
}

type ActionTypeData struct {
	Task *constant.Task
}

func New(name string) *PrintWifsType {
	t := &PrintWifsType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *PrintWifsType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
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
			result, err := p.do(task)
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
				Data:     result,
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

func (p *PrintWifsType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *PrintWifsType) init(task *constant.Task) error {
	var config Config
	err := go_format.FormatInstance.MapToStruct(&config, task.Data)
	if err != nil {
		return err
	}
	p.config = &config

	p.btcWallet = go_coin_btc.NewWallet(&chaincfg.MainNetParams)
	return nil
}

func (p *PrintWifsType) do(task *constant.Task) (interface{}, error) {

	addresses := make([]*constant.BtcAddress, 0)
	err := go_mysql.MysqlInstance.RawSelect(
		&addresses,
		p.config.SelectAddressSql[0],
		p.config.SelectAddressSql[1],
	)
	if err != nil {
		return "", err
	}
	seedPass, err := go_crypto.CryptoInstance.AesCbcDecrypt(global.GlobalConfig.Pass, p.config.Pass)
	if err != nil {
		return "", err
	}
	seedHex := p.btcWallet.SeedHexByMnemonic(p.config.Mnemonic, seedPass)
	wifs := make([]string, 0)
	for _, addrDb := range addresses {
		keyInfo, err := p.btcWallet.DeriveBySeedPath(seedHex, fmt.Sprintf("m/86'/0'/0'/0/%d", addrDb.Index))
		if err != nil {
			return "", err
		}
		wifs = append(wifs, keyInfo.Wif)
	}

	return strings.Join(wifs, "\n"), nil
}
