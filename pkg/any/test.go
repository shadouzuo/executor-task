package any

import (
	"context"
	"time"

	"github.com/pkg/errors"

	go_best_type "github.com/pefish/go-best-type"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/db"
)

type TestType struct {
	go_best_type.BaseBestType
}

func NewTestType(bestTypeManager *go_best_type.BestTypeManager) *TestType {
	t := &TestType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, bestTypeManager, 0)
	return t
}

type ActionType_Test_Data struct {
	Task *db.Task
}

func (p *TestType) Start(stopCtx context.Context, terminalCtx context.Context, ask *go_best_type.AskType) {
	task := ask.Data.(ActionType_Test_Data).Task
	newStatus, err := p.doLoop(stopCtx, terminalCtx, task)
	if err != nil {
		_, rowsAffected, err := go_mysql.MysqlInstance.Update(
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
		if err != nil {
			p.Logger().Error(err)
			return
		}
		if rowsAffected == 0 {
			p.Logger().Error(errors.New("Update error."))
			return
		}
		return
	}
	_, rowsAffected, err := go_mysql.MysqlInstance.Update(
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
		return
	}
	if rowsAffected == 0 {
		p.Logger().Error(errors.New("Update error."))
		return
	}
	p.Logger().Info("Start exited")
}

func (p *TestType) ProcessOtherAsk(stopCtx context.Context, terminalCtx context.Context, ask *go_best_type.AskType) {
}

func (p *TestType) doLoop(stopCtx context.Context, terminalCtx context.Context, task *db.Task) (constant.TaskStatusType, error) {
	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			err := p.do(task)
			if err != nil {
				return constant.TaskStatusType_ExitedWithErr, err
			}
			if task.Interval == 0 {
				return constant.TaskStatusType_Exited, nil
			} else {
				timer.Reset(time.Duration(task.Interval) * time.Second)
			}
		case <-stopCtx.Done():
			p.Logger().InfoF("Exited by user.")
			return constant.TaskStatusType_Exited, nil
		case <-terminalCtx.Done():
			p.Logger().InfoF("Exited by system.")
			return constant.TaskStatusType_WaitExec, nil
		}
	}
}

func (p *TestType) do(task *db.Task) error {
	p.Logger().InfoF("<%s> test...", task.Name)
	return nil
}

func (p *TestType) Name() string {
	return "Test"
}
