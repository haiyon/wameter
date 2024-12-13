package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Database database config struct
type Database struct {
	Master   *DBNode   `mapstructure:"master"`
	Slaves   []*DBNode `mapstructure:"slaves"`
	Migrate  bool      `mapstructure:"migrate"`
	Strategy string    `mapstructure:"strategy"`
	MaxRetry int       `mapstructure:"max_retry"`
}

// DBNode represents single database node configuration
type DBNode struct {
	Driver          string        `mapstructure:"driver"`
	Source          string        `mapstructure:"source"`
	Logging         bool          `mapstructure:"logging"`
	MaxIdleConn     int           `mapstructure:"max_idle_conn"`
	MaxOpenConn     int           `mapstructure:"max_open_conn"`
	ConnMaxLifeTime time.Duration `mapstructure:"conn_max_life_time"`
	Weight          int           `mapstructure:"weight"`
}

// getSlaveConfigs reads slave database configurations
func getSlaveConfigs(v *viper.Viper) []*DBNode {
	var slaves []*DBNode

	slavesConfig := v.Get("data.database.slaves")
	if slavesConfig == nil {
		return slaves
	}

	slavesList, ok := slavesConfig.([]any)
	if !ok {
		return slaves
	}

	slavesCount := len(slavesList)
	for i := 0; i < slavesCount; i++ {
		slave := &DBNode{
			Driver:          v.GetString(fmt.Sprintf("data.database.slaves.%d.driver", i)),
			Source:          v.GetString(fmt.Sprintf("data.database.slaves.%d.source", i)),
			Logging:         v.GetBool(fmt.Sprintf("data.database.slaves.%d.logging", i)),
			MaxIdleConn:     v.GetInt(fmt.Sprintf("data.database.slaves.%d.max_idle_conn", i)),
			MaxOpenConn:     v.GetInt(fmt.Sprintf("data.database.slaves.%d.max_open_conn", i)),
			ConnMaxLifeTime: v.GetDuration(fmt.Sprintf("data.database.slaves.%d.max_life_time", i)),
			Weight:          v.GetInt(fmt.Sprintf("data.database.slaves.%d.weight", i)),
		}
		slaves = append(slaves, slave)
	}
	return slaves
}
