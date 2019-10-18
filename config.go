package main

import (
	"os"
	"strconv"
	"time"

	"github.com/pingcap/errors"
	"github.com/siddontang/go-mysql/client"
	"github.com/siddontang/go-mysql/mysql"
)

type Configuration struct {
	General GeneralConfig `toml:"general"`
	MySQL   MySQLConfig   `toml:"mysql"`
	Metrics MetricsConfig `toml:"metrics"`
}

type GeneralConfig struct {
	ListenAddress string `toml:"listen_address"`
	LogFile       string `toml:"log_file"`
	LogLevel      string `toml:"log_level"`
}

type MySQLConfig struct {
	Host     string `toml:"host"`
	Port     uint16 `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	ServerID uint32 `toml:"server_id"`
}

type MetricsConfig struct {
	ListenAddress string `toml:"listen_address"`
	Endpoint      string `toml:"endpoint"`
	SSLVerify     bool   `toml:"ssl_verify"`
}

func newConfiguration() *Configuration {
	serverID := uint32(time.Now().Unix())
	serverID ^= uint32(os.Getpid())
	return &Configuration{
		General: GeneralConfig{
			ListenAddress: "0.0.0.0:999",
			LogFile:       "/var/log/proxysql_binlog/proxysql_binlog.log",
			LogLevel:      "info",
		},
		MySQL: MySQLConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "rslave",
			Password: "RQL0jLL8lO6rLl",
			ServerID: serverID,
		},
		Metrics: MetricsConfig{
			ListenAddress: "0.0.0.0:9056",
			Endpoint:      "metrics",
		},
	}
}

func (c *Configuration) getMasterGTIDSet() (mysql.GTIDSet, error) {
	query := "SELECT @@GLOBAL.GTID_EXECUTED"
	rr, err := c.execute(query)
	if err != nil {
		return nil, errors.Trace(err)
	}
	gx, err := rr.GetString(0, 0)
	if err != nil {
		return nil, errors.Trace(err)
	}
	gset, err := mysql.ParseMysqlGTIDSet(gx)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return gset, nil
}

func (c *Configuration) execute(cmd string, args ...interface{}) (rr *mysql.Result, err error) {
	var conn *client.Conn
	retryNum := 3
	for i := 0; i < retryNum; i++ {
		conn, err = client.Connect(c.MySQL.Host+":"+strconv.Itoa(int(c.MySQL.Port)), c.MySQL.User, c.MySQL.Password, "")
		if err != nil {
			return nil, errors.Trace(err)
		}

		rr, err = conn.Execute(cmd, args...)
		if err != nil && !mysql.ErrorEqual(err, mysql.ErrBadConn) {
			return
		} else if mysql.ErrorEqual(err, mysql.ErrBadConn) {
			conn.Close()
			continue
		} else {
			conn.Close()
			return
		}
	}
	return
}
