package test

import (
	"context"
	"time"

	i_logger "github.com/pefish/go-interface/i-logger"
	"github.com/shadouzuo/executor-task/pkg/constant"
)

type TestType struct {
	logger i_logger.ILogger
}

func New(logger i_logger.ILogger) *TestType {
	t := &TestType{
		logger: logger,
	}
	return t
}

func (p *TestType) Start(ctx context.Context, task *constant.Task) (any, error) {
	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			err := p.do(task)
			if err != nil {
				return nil, err
			}
			if task.Interval != 0 {
				timer.Reset(time.Duration(task.Interval) * time.Second)
				continue
			}
			return nil, nil
		case <-ctx.Done():
			return nil, nil
		}
	}
}

func (p *TestType) do(task *constant.Task) error {
	p.logger.InfoF("<%s> test...", task.Name)
	return nil
}
