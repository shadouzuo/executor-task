package any

import (
	"time"

	go_best_type "github.com/pefish/go-best-type"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/db"
)

type TestType struct {
	go_best_type.BaseBestType
}

func NewTestType(name string) *TestType {
	t := &TestType{}
	t.BaseBestType = *go_best_type.NewBaseBestType(t, name)
	return t
}

type ActionTypeData struct {
	Task *db.Task
}

func (p *TestType) Start(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	task := ask.Data.(ActionTypeData).Task
	newStatus, err := p.doLoop(exitChan, task)
	if err != nil {
		_, err := go_mysql.MysqlInstance.Update(
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
			return err
		}
		return nil
	}
	_, err = go_mysql.MysqlInstance.Update(
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
		return err
	}
	return nil
}

func (p *TestType) ProcessOtherAsk(exitChan <-chan go_best_type.ExitType, ask *go_best_type.AskType) error {
	return nil
}

func (p *TestType) doLoop(exitChan <-chan go_best_type.ExitType, task *db.Task) (constant.TaskStatusType, error) {
	timer := time.NewTimer(0)
	for {
		select {
		case <-timer.C:
			err := p.do(task)
			if err != nil {
				return constant.TaskStatusType_ExitedWithErr, err
			}
			if task.Interval != 0 {
				timer.Reset(time.Duration(task.Interval) * time.Second)
			} else {
				go p.BestTypeManager().ExitOne(p.Name(), go_best_type.ExitType_User)
			}
		case exitType := <-exitChan:
			switch exitType {
			case go_best_type.ExitType_System:
				p.Logger().InfoF("Exited by system.")
				return constant.TaskStatusType_WaitExec, nil
			case go_best_type.ExitType_User:
				p.Logger().InfoF("Exited by user.")
				return constant.TaskStatusType_Exited, nil
			}
		}
	}
}

func (p *TestType) do(task *db.Task) error {
	p.Logger().InfoF("<%s> test...", task.Name)
	return nil
}
