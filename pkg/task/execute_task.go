package task

import (
	"context"
	"time"

	go_best_type "github.com/pefish/go-best-type"
	go_logger "github.com/pefish/go-logger"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/pkg/errors"
	any "github.com/shadouzuo/executor-task/pkg/any"
	distribute_btc "github.com/shadouzuo/executor-task/pkg/any/distribute-btc"
	gather_btc "github.com/shadouzuo/executor-task/pkg/any/gather-btc"
	update_btc_confirm "github.com/shadouzuo/executor-task/pkg/any/update-btc-confirm"
	update_btc_utxo "github.com/shadouzuo/executor-task/pkg/any/update-btc-utxo"
	constant "github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/db"
)

type ExecuteTask struct {
	logger          go_logger.InterfaceLogger
	bestTypeManager *go_best_type.BestTypeManager
}

func NewExecuteTask() *ExecuteTask {
	w := &ExecuteTask{}
	w.logger = go_logger.Logger.CloneWithPrefix(w.Name())
	w.bestTypeManager = go_best_type.NewBestTypeManager(w.logger)
	return w
}

func (t *ExecuteTask) Init(ctx context.Context) error {
	return nil
}

func (t *ExecuteTask) Run(ctx context.Context) error {
	tasks := make([]*db.Task, 0)

	err := go_mysql.MysqlInstance.Select(
		&tasks,
		&go_mysql.SelectParams{
			TableName: "task",
			Select:    "*",
			Where: map[string]interface{}{
				"status": []constant.TaskStatusType{
					constant.TaskStatusType_WaitExec,
					constant.TaskStatusType_WaitExit,
				},
			},
		},
	)
	if err != nil {
		return err
	}

	// if len(tasks) == 0 {
	// 	t.logger.InfoF("Nothing.")
	// 	return nil
	// }

	for _, task := range tasks {
		_, err := go_mysql.MysqlInstance.Update(
			&go_mysql.UpdateParams{
				TableName: "task",
				Update: map[string]interface{}{
					"status": constant.TaskStatusType_Executing,
				},
				Where: map[string]interface{}{
					"id": task.Id,
				},
			},
		)
		if err != nil {
			return err
		}
		err = t.execTask(task)
		if err != nil {
			t.logger.Error(err)
			_, err := go_mysql.MysqlInstance.Update(
				&go_mysql.UpdateParams{
					TableName: "task",
					Update: map[string]interface{}{
						"status": constant.TaskStatusType_ExitedWithErr,
						"mark":   err.Error(),
					},
					Where: map[string]interface{}{
						"id": task.Id,
					},
				},
			)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (t *ExecuteTask) execTask(task *db.Task) error {
	switch task.Status {
	case constant.TaskStatusType_WaitExec:
		var bestType go_best_type.IBestType

		switch task.Name {
		case "test":
			bestType = any.NewTestType(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: any.ActionTypeData{
					Task: task,
				},
			})
		case "gene_btc_address":
			bestType = any.NewGeneBtcAddressType(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: any.ActionTypeData{
					Task: task,
				},
			})
		case "distribute_btc":
			bestType = distribute_btc.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: distribute_btc.ActionTypeData{
					Task: task,
				},
			})
		case "update_btc_utxo":
			bestType = update_btc_utxo.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: update_btc_utxo.ActionTypeData{
					Task: task,
				},
			})
		case "update_btc_confirm":
			bestType = update_btc_confirm.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: update_btc_confirm.ActionTypeData{
					Task: task,
				},
			})
		case "gather_btc":
			bestType = gather_btc.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: gather_btc.ActionTypeData{
					Task: task,
				},
			})
		default:
			return errors.New("Task not be supported.")
		}
		t.logger.InfoF("Task <%s> executing.", task.Name)
	case constant.TaskStatusType_WaitExit:
		t.logger.InfoF("Task <%s> exiting.", task.Name)
		t.bestTypeManager.ExitOne(task.Name, go_best_type.ExitType_User)
	}
	return nil
}

func (t *ExecuteTask) Stop() error {
	t.bestTypeManager.ExitAll(go_best_type.ExitType_System)
	return nil
}

func (t *ExecuteTask) Name() string {
	return "ExecuteTask"
}

func (t *ExecuteTask) Interval() time.Duration {
	return 5 * time.Second
}

func (t *ExecuteTask) Logger() go_logger.InterfaceLogger {
	return t.logger
}
