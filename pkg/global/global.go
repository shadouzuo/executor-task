package global

import "github.com/pefish/go-commander"

type Config struct {
	commander.BasicConfig
	DbHost string `json:"db-host" default:"mysql" usage:"Database host."`
	DbPort int    `json:"db-port" default:"3306" usage:"Database port."`
	DbDb   string `json:"db-db" default:"" usage:"Database to connect."`
	DbUser string `json:"db-user" default:"admin" usage:"Username to connect database."`
	DbPass string `json:"db-pass" default:"" usage:"Password to connect database."`
	Pass   string `json:"pass" default:"" usage:"Password to decrypt sth."`
}

var GlobalConfig Config
