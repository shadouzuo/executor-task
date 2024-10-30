package gene_jwt_key

import (
	"context"
	"fmt"
	"time"

	go_crypto "github.com/pefish/go-crypto"
	go_format "github.com/pefish/go-format"
	i_logger "github.com/pefish/go-interface/i-logger"
	"github.com/shadouzuo/executor-task/pkg/constant"
)

type GeneJwtKeyType struct {
	logger i_logger.ILogger
}

type ActionTypeData struct {
	Task *constant.Task
}

type GeneJwtKeyTypeConfig struct {
}

func New(logger i_logger.ILogger) *GeneJwtKeyType {
	t := &GeneJwtKeyType{
		logger: logger,
	}
	return t
}

func (p *GeneJwtKeyType) Start(ctx context.Context, task *constant.Task) (any, error) {
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

func (p *GeneJwtKeyType) do(task *constant.Task) (interface{}, error) {
	var config GeneJwtKeyTypeConfig
	err := go_format.MapToStruct(&config, task.Data)
	if err != nil {
		return "", err
	}

	priv, pubk := go_crypto.MustGeneRsaKeyPair()
	fmt.Println(priv)
	fmt.Println(pubk)
	return map[string]interface{}{
		"priv": priv,
		"pubk": pubk,
	}, nil
}
