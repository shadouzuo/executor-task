package main

import (
	"log"

	"github.com/pefish/go-commander"
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
		log.Fatal(err)
	}
}
