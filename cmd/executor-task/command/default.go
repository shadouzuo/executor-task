package command

import (
	"github.com/shadouzuo/executor-task/pkg/global"
	"github.com/shadouzuo/executor-task/pkg/task"

	"github.com/pefish/go-commander"
	go_logger "github.com/pefish/go-logger"
	go_mysql "github.com/pefish/go-mysql"
	task_driver "github.com/pefish/go-task-driver"
)

type DefaultCommand struct {
}

func NewDefaultCommand() *DefaultCommand {
	return &DefaultCommand{}
}

func (dc *DefaultCommand) Config() interface{} {
	return &global.GlobalConfig
}

func (dc *DefaultCommand) Data() interface{} {
	return nil
}

func (dc *DefaultCommand) Init(command *commander.Commander) error {

	go_mysql.MysqlInstance.SetLogger(go_logger.Logger)
	err := go_mysql.MysqlInstance.ConnectWithConfiguration(go_mysql.Configuration{
		Host:     global.GlobalConfig.DbHost,
		Username: global.GlobalConfig.DbUser,
		Password: global.GlobalConfig.DbPass,
		Database: global.GlobalConfig.DbDb,
	})
	if err != nil {
		return err
	}

	return nil
}

func (dc *DefaultCommand) OnExited(command *commander.Commander) error {
	go_mysql.MysqlInstance.Close()
	return nil
}

func (dc *DefaultCommand) Start(command *commander.Commander) error {
	taskDriver := task_driver.NewTaskDriver()
	taskDriver.Register(task.NewExecuteTask())
	taskDriver.RunWait(command.Ctx)
	return nil
}
