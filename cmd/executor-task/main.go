package main

import (
	"github.com/pefish/go-commander"
	go_logger "github.com/pefish/go-logger"
	"github.com/shadouzuo/executor-task/cmd/executor-task/command"
	"github.com/shadouzuo/executor-task/version"
)

func main() {
	commanderInstance := commander.NewCommander(version.AppName, version.Version, version.AppName+" is a template.")
	commanderInstance.RegisterDefaultSubcommand(&commander.SubcommandInfo{
		Desc:       "Use this command by default if you don't set subcommand.",
		Args:       nil,
		Subcommand: command.NewDefaultCommand(),
	})
	err := commanderInstance.Run()
	if err != nil {
		go_logger.Logger.Error(err)
	}
}
