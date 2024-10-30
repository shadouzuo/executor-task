package command

import (
	"github.com/shadouzuo/executor-task/pkg/global"
	"github.com/shadouzuo/executor-task/pkg/task"

	"github.com/pefish/go-commander"
	go_config "github.com/pefish/go-config"
	t_mysql "github.com/pefish/go-interface/t-mysql"
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
	global.MysqlInstance = go_mysql.NewMysqlInstance(command.Logger)
	err := global.MysqlInstance.ConnectWithConfiguration(t_mysql.Configuration{
		Host:     global.GlobalConfig.DbHost,
		Username: global.GlobalConfig.DbUser,
		Password: global.GlobalConfig.DbPass,
		Database: global.GlobalConfig.DbDb,
	})
	if err != nil {
		return err
	}

	err = go_config.FetchConfigsFromDb(&global.GlobalConfigInDb, global.MysqlInstance)
	if err != nil {
		return err
	}

	return nil
}

func (dc *DefaultCommand) OnExited(command *commander.Commander) error {
	global.MysqlInstance.Close()
	return nil
}

func (dc *DefaultCommand) Start(command *commander.Commander) error {
	taskDriver := task_driver.NewTaskDriver()
	taskDriver.Register(task.NewExecuteTask(command.Logger))
	taskDriver.RunWait(command.Ctx)
	return nil
}
