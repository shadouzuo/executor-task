package gene_jwt_key

import (
	"errors"
	"fmt"
	"time"

	go_best_type "github.com/pefish/go-best-type"
	go_crypto "github.com/pefish/go-crypto"
	go_format "github.com/pefish/go-format"
	"github.com/shadouzuo/executor-task/pkg/constant"
)

type GeneJwtKeyType struct {
	go_best_type.BaseBestType
}

type ActionTypeData struct {
	Task *constant.Task
}

type GeneJwtKeyTypeConfig struct {
}

func New(name string) *GeneJwtKeyType {
	t := &GeneJwtKeyType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

func (p *GeneJwtKeyType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
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

func (p *GeneJwtKeyType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *GeneJwtKeyType) do(task *constant.Task) (interface{}, error) {
	var config GeneJwtKeyTypeConfig
	err := go_format.FormatInstance.MapToStruct(&config, task.Data)
	if err != nil {
		return "", err
	}

	priv, pubk := go_crypto.CryptoInstance.MustGeneRsaKeyPair()
	fmt.Println(priv)
	fmt.Println(pubk)
	return map[string]interface{}{
		"priv": priv,
		"pubk": pubk,
	}, nil
}
