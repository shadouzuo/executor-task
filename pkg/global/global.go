package global

import (
	"github.com/pefish/go-commander"
	go_mysql "github.com/pefish/go-mysql"
)

type Config struct {
	commander.BasicConfig
	DbHost string `json:"db-host" default:"mysql" usage:"Database host."`
	DbPort int    `json:"db-port" default:"3306" usage:"Database port."`
	DbDb   string `json:"db-db" default:"" usage:"Database to connect."`
	DbUser string `json:"db-user" default:"admin" usage:"Username to connect database."`
	DbPass string `json:"db-pass" default:"" usage:"Password to connect database."`
}

type ConfigInDb struct {
	Pass string `json:"pass" default:"" usage:"Password to decrypt sth."`
}

var GlobalConfigInDb ConfigInDb

var GlobalConfig Config

var MysqlInstance *go_mysql.MysqlType
