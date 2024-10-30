package task

import (
	"context"
	"time"

	go_crypto "github.com/pefish/go-crypto"
	go_format "github.com/pefish/go-format"
	i_logger "github.com/pefish/go-interface/i-logger"
	t_mysql "github.com/pefish/go-interface/t-mysql"
	"github.com/pkg/errors"
	any_ "github.com/shadouzuo/executor-task/pkg/any"
	build_btc_tx "github.com/shadouzuo/executor-task/pkg/any/build-btc-tx"
	"github.com/shadouzuo/executor-task/pkg/any/check_mbtc_transfer"
	distribute_btc "github.com/shadouzuo/executor-task/pkg/any/distribute-btc"
	distribute_evm_token "github.com/shadouzuo/executor-task/pkg/any/distribute-evm-token"
	gather_btc "github.com/shadouzuo/executor-task/pkg/any/gather-btc"
	gene_btc_address "github.com/shadouzuo/executor-task/pkg/any/gene-btc-address"
	gene_jwt_key "github.com/shadouzuo/executor-task/pkg/any/gene-jwt-key"
	merlin_print_aa_address "github.com/shadouzuo/executor-task/pkg/any/merlin-print-aa-address"
	print_eth_address "github.com/shadouzuo/executor-task/pkg/any/print-eth-address"
	print_tg_group_id "github.com/shadouzuo/executor-task/pkg/any/print-tg-group-id"
	print_wifs "github.com/shadouzuo/executor-task/pkg/any/print-wifs"
	"github.com/shadouzuo/executor-task/pkg/any/test"
	transfer_btc "github.com/shadouzuo/executor-task/pkg/any/transfer-btc"
	update_btc_confirm "github.com/shadouzuo/executor-task/pkg/any/update-btc-confirm"
	update_btc_utxo "github.com/shadouzuo/executor-task/pkg/any/update-btc-utxo"
	constant "github.com/shadouzuo/executor-task/pkg/constant"
	"github.com/shadouzuo/executor-task/pkg/global"
)

type ExecuteTask struct {
	logger  i_logger.ILogger
	cancels map[string]context.CancelFunc
}

func NewExecuteTask(logger i_logger.ILogger) *ExecuteTask {
	w := &ExecuteTask{
		cancels: make(map[string]context.CancelFunc, 0),
	}
	w.logger = logger.CloneWithPrefix(w.Name())
	return w
}

func (t *ExecuteTask) Init(ctx context.Context) error {
	return nil
}

func (t *ExecuteTask) Run(ctx context.Context) error {
	tasks := make([]*constant.Task, 0)

	err := global.MysqlInstance.Select(
		&tasks,
		&t_mysql.SelectParams{
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
		t.logger.InfoF("No task to execute.")
		return nil
	}

	for _, task := range tasks {
		_, err := global.MysqlInstance.Update(
			&t_mysql.UpdateParams{
				TableName: "task",
				Update: map[string]interface{}{
					"data":   "{}",
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
		newCtx, cancel := context.WithCancel(ctx)
		t.cancels[task.Name] = cancel
		switch task.Status {
		case constant.TaskStatusType_WaitExec:
			switch task.Name {
			case "test":
				t.execTaskAsync(newCtx, test.New(t.logger), task)
			case "gene_btc_address":
				t.execTaskAsync(newCtx, gene_btc_address.New(t.logger), task)
			case "distribute_btc":
				t.execTaskAsync(newCtx, distribute_btc.New(t.logger), task)
			case "update_btc_utxo":
				t.execTaskAsync(newCtx, update_btc_utxo.New(t.logger), task)
			case "update_btc_confirm":
				t.execTaskAsync(newCtx, update_btc_confirm.New(t.logger), task)
			case "gather_btc":
				t.execTaskAsync(newCtx, gather_btc.New(t.logger), task)
			case "build_btc_tx":
				t.execTaskAsync(newCtx, build_btc_tx.New(t.logger), task)
			case "transfer_btc":
				t.execTaskAsync(newCtx, transfer_btc.New(t.logger), task)
			case "print_wifs":
				t.execTaskAsync(newCtx, print_wifs.New(t.logger), task)
			case "print_tg_group_id":
				t.execTaskAsync(newCtx, print_tg_group_id.New(t.logger), task)
			case "print_eth_address":
				t.execTaskAsync(newCtx, print_eth_address.New(t.logger), task)
			case "merlin_print_aa_address":
				t.execTaskAsync(newCtx, merlin_print_aa_address.New(t.logger), task)
			case "distribute_evm_token":
				t.execTaskAsync(newCtx, distribute_evm_token.New(t.logger), task)
			case "check_mbtc_transfer":
				t.execTaskAsync(newCtx, check_mbtc_transfer.New(t.logger), task)
			case "gene_jwt_key":
				t.execTaskAsync(newCtx, gene_jwt_key.New(t.logger), task)
			default:
				return errors.New("Task not be supported.")
			}
			t.logger.InfoF("Task <%s> executing.", task.Name)
		case constant.TaskStatusType_WaitExit:
			t.logger.InfoF("Task <%s> exiting.", task.Name)
			t.cancels[task.Name]()
		}
	}

	return nil
}

func (t *ExecuteTask) execTaskAsync(
	ctx context.Context,
	executor any_.IExecutor,
	task *constant.Task,
) {
	go func() {
		result, err := executor.Start(ctx, task)
		t.Logger().InfoF("<%s> 执行完成.", task.Name)
		newStatus := constant.TaskStatusType_Exited
		mark := go_format.ToString(result)
		if err != nil {
			t.logger.ErrorF("<%s> 执行错误. %+v", task.Name, err)
			newStatus = constant.TaskStatusType_ExitedWithErr
			mark = err.Error()
		}
		_, err = global.MysqlInstance.Update(
			&t_mysql.UpdateParams{
				TableName: "task",
				Update: map[string]interface{}{
					"data":   "{}",
					"status": newStatus,
					"mark":   mark,
				},
				Where: map[string]interface{}{
					"id": task.Id,
				},
			},
		)
		if err != nil {
			t.logger.Error(err)
			return
		}
		_, err = global.MysqlInstance.Insert(
			"task_record",
			constant.TaskRecord{
				Name:     task.Name,
				Interval: task.Interval,
				Data: map[string]interface{}{
					"data": go_crypto.MustAesCbcEncrypt(global.GlobalConfig.Pass, go_format.ToString(task.Data)),
				},
				Mark: go_crypto.MustAesCbcEncrypt(global.GlobalConfig.Pass, mark),
			},
		)
		if err != nil {
			t.logger.Error(err)
			return
		}
	}()

}

func (t *ExecuteTask) Stop() error {
	// 将所有正在运行的任务状态改成等待运行

	return nil
}

func (t *ExecuteTask) Name() string {
	return "ExecuteTask"
}

func (t *ExecuteTask) Interval() time.Duration {
	return 5 * time.Second
}

func (t *ExecuteTask) Logger() i_logger.ILogger {
	return t.logger
}
