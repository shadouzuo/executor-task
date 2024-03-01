package task

import (
	"context"
	"time"

	go_best_type "github.com/pefish/go-best-type"
	go_logger "github.com/pefish/go-logger"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/pkg/errors"
	any "github.com/shadouzuo/executor-task/pkg/any"
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
	w.bestTypeManager = go_best_type.NewBestTypeManager()
	return w
}

func (t *ExecuteTask) Init(ctx context.Context) error {
	return nil
}

func (t *ExecuteTask) Run(ctx context.Context) error {
	tasks := make([]db.Task, 0)

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

	if len(tasks) == 0 {
		t.logger.InfoF("Nothing.")
		return nil
	}

	for _, task := range tasks {
		_, rowsAffected, err := go_mysql.MysqlInstance.Update(
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
		if rowsAffected == 0 {
			return errors.New("Update error.")
		}

		switch task.Status {
		case constant.TaskStatusType_WaitExec:
			t.ExecTask(ctx, &task)
		case constant.TaskStatusType_WaitExit:
			t.ExitTask(ctx, &task)
		}

	}

	return nil
}

func (t *ExecuteTask) ExecTask(ctx context.Context, task *db.Task) error {
	testInstance := t.bestTypeManager.Get(task.Name)
	if testInstance == nil {
		testInstance = any.NewTestType(t.bestTypeManager)
		t.bestTypeManager.Set(task.Name, testInstance)
	}

	t.logger.InfoF("to start")
	testInstance.Ask(&go_best_type.AskType{
		Action: constant.ActionType_Start,
		Data: any.ActionType_Test_Data{
			Task: task,
		},
	})
	t.logger.InfoF("Task <%d> executing.", task.Name)
	return nil
}

func (t *ExecuteTask) ExitTask(ctx context.Context, task *db.Task) error {
	t.bestTypeManager.StopOneAsync(task.Name)
	t.logger.InfoF("Task <%d> stopping.", task.Name)
	return nil
}

func (t *ExecuteTask) Stop() error {
	t.bestTypeManager.TerminalAllAsync()
	t.bestTypeManager.Wait()
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
