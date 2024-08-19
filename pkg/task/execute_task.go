package task

import (
	"context"
	"time"

	go_best_type "github.com/pefish/go-best-type"
	go_crypto "github.com/pefish/go-crypto"
	go_format "github.com/pefish/go-format"
	go_logger "github.com/pefish/go-logger"
	go_mysql "github.com/pefish/go-mysql"
	"github.com/pkg/errors"
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
	logger          go_logger.InterfaceLogger
	bestTypeManager *go_best_type.BestTypeManager
	taskResultChan  chan interface{}
}

func NewExecuteTask() *ExecuteTask {
	w := &ExecuteTask{}
	w.logger = go_logger.Logger.CloneWithPrefix(w.Name())
	w.bestTypeManager = go_best_type.NewBestTypeManager(w.logger)
	w.taskResultChan = make(chan interface{})
	return w
}

func (t *ExecuteTask) Init(ctx context.Context) error {
	// 等任务执行结果的
	go func() {
		for {
			select {
			case r := <-t.taskResultChan:
				taskResult := r.(constant.TaskResult)
				t.Logger().InfoF("<%s> 执行完成.", taskResult.Task.Name)
				newStatus := constant.TaskStatusType_Exited
				mark := go_crypto.CryptoInstance.MustAesCbcEncrypt(global.GlobalConfig.Pass, go_format.FormatInstance.ToString(taskResult.Data))
				if taskResult.Err != nil {
					if taskResult.Err.Error() == "Exited by system." {
						newStatus = constant.TaskStatusType_WaitExec
					} else if taskResult.Err.Error() == "Exited by user." {
						newStatus = constant.TaskStatusType_Exited
					} else {
						t.logger.ErrorF("<%s> 执行错误. %+v", taskResult.Task.Name, taskResult.Err)
						newStatus = constant.TaskStatusType_ExitedWithErr
					}
					mark = taskResult.Err.Error()
				}
				_, err := go_mysql.MysqlInstance.Update(
					&go_mysql.UpdateParams{
						TableName: "task",
						Update: map[string]interface{}{
							"data":   "{}",
							"status": newStatus,
							"mark":   mark,
						},
						Where: map[string]interface{}{
							"id": taskResult.Task.Id,
						},
					},
				)
				if err != nil {
					t.logger.Error(err)
					continue
				}
				_, err = go_mysql.MysqlInstance.Insert(
					"task_record",
					constant.TaskRecord{
						Name:     taskResult.Task.Name,
						Interval: taskResult.Task.Interval,
						Data: map[string]interface{}{
							"data": go_crypto.CryptoInstance.MustAesCbcEncrypt(global.GlobalConfig.Pass, go_format.FormatInstance.ToString(taskResult.Task.Data)),
						},
						Mark: mark,
					},
				)
				if err != nil {
					t.logger.Error(err)
					continue
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (t *ExecuteTask) Run(ctx context.Context) error {
	tasks := make([]*constant.Task, 0)

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
		t.logger.InfoF("No task to execute.")
		return nil
	}

	for _, task := range tasks {
		_, err := go_mysql.MysqlInstance.Update(
			&go_mysql.UpdateParams{
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

func (t *ExecuteTask) execTask(task *constant.Task) error {
	switch task.Status {
	case constant.TaskStatusType_WaitExec:
		var bestType go_best_type.IBestType

		switch task.Name {
		case "test":
			bestType = test.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: test.ActionTypeData{
					Task: task,
				},
				AnswerChan: t.taskResultChan,
			})
		case "gene_btc_address":
			bestType = gene_btc_address.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: gene_btc_address.ActionTypeData{
					Task: task,
				},
				AnswerChan: t.taskResultChan,
			})
		case "distribute_btc":
			bestType = distribute_btc.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: distribute_btc.ActionTypeData{
					Task: task,
				},
				AnswerChan: t.taskResultChan,
			})
		case "update_btc_utxo":
			bestType = update_btc_utxo.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: update_btc_utxo.ActionTypeData{
					Task: task,
				},
				AnswerChan: t.taskResultChan,
			})
		case "update_btc_confirm":
			bestType = update_btc_confirm.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: update_btc_confirm.ActionTypeData{
					Task: task,
				},
				AnswerChan: t.taskResultChan,
			})
		case "gather_btc":
			bestType = gather_btc.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: gather_btc.ActionTypeData{
					Task: task,
				},
				AnswerChan: t.taskResultChan,
			})
		case "build_btc_tx":
			bestType = build_btc_tx.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: build_btc_tx.ActionTypeData{
					Task: task,
				},
				AnswerChan: t.taskResultChan,
			})
		case "transfer_btc":
			bestType = transfer_btc.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: transfer_btc.ActionTypeData{
					Task: task,
				},
				AnswerChan: t.taskResultChan,
			})
		case "print_wifs":
			bestType = print_wifs.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: print_wifs.ActionTypeData{
					Task: task,
				},
				AnswerChan: t.taskResultChan,
			})
		case "print_tg_group_id":
			bestType = print_tg_group_id.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: print_tg_group_id.ActionTypeData{
					Task: task,
				},
				AnswerChan: t.taskResultChan,
			})
		case "print_eth_address":
			bestType = print_eth_address.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: print_eth_address.ActionTypeData{
					Task: task,
				},
				AnswerChan: t.taskResultChan,
			})
		case "merlin_print_aa_address":
			bestType = merlin_print_aa_address.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: merlin_print_aa_address.ActionTypeData{
					Task: task,
				},
				AnswerChan: t.taskResultChan,
			})
		case "distribute_evm_token":
			bestType = distribute_evm_token.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: distribute_evm_token.ActionTypeData{
					Task: task,
				},
				AnswerChan: t.taskResultChan,
			})
		case "check_mbtc_transfer":
			bestType = check_mbtc_transfer.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: check_mbtc_transfer.ActionTypeData{
					Task: task,
				},
				AnswerChan: t.taskResultChan,
			})
		case "gene_jwt_key":
			bestType = gene_jwt_key.New(task.Name)
			t.bestTypeManager.Set(bestType)
			bestType.Ask(&go_best_type.AskType{
				Action: constant.ActionType_Start,
				Data: gene_jwt_key.ActionTypeData{
					Task: task,
				},
				AnswerChan: t.taskResultChan,
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
