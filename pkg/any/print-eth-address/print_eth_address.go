package print_eth_address

import (
	"errors"
	"time"

	go_best_type "github.com/pefish/go-best-type"
	go_coin_eth "github.com/pefish/go-coin-eth"
	go_format "github.com/pefish/go-format"
	"github.com/shadouzuo/executor-task/pkg/constant"
)

type PrintEthAddressType struct {
	go_best_type.BaseBestType
}

type ActionTypeData struct {
	Task *constant.Task
}

type PrintEthAddressConfig struct {
	Mnemonic string `json:"mnemonic"`
	Pass     string `json:"pass"`
	Path     string `json:"path"`
}

func New(name string) *PrintEthAddressType {
	t := &PrintEthAddressType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *PrintEthAddressType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	task := ask.Data.(ActionTypeData).Task

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

func (p *PrintEthAddressType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *PrintEthAddressType) do(task *constant.Task) (interface{}, error) {
	var config PrintEthAddressConfig
	err := go_format.FormatInstance.MapToStruct(&config, task.Data)
	if err != nil {
		return "", err
	}

	wallet := go_coin_eth.NewWallet()
	seed := wallet.SeedHexByMnemonic(config.Mnemonic, config.Pass)

	result, err := wallet.DeriveFromPath(seed, config.Path)
	if err != nil {
		return "", err
	}

	return map[string]interface{}{
		"address": result.Address,
		"priv":    result.PrivateKey,
	}, nil
}
