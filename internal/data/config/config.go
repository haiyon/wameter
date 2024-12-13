package config

import "github.com/spf13/viper"

// Config data config struct
type Config struct {
	Enveronment string `mapstructure:"environment"`
	*Database
	*Redis
	*Meilisearch
	*Elasticsearch
	*MongoDB
	*Neo4j
	*RabbitMQ
	*Kafka
}

// GetConfig reads data configurations
func GetConfig(v *viper.Viper) *Config {
	return &Config{
		Enveronment: v.GetString("data.environment"),
		Database: &Database{
			Master: &DBNode{
				Driver:          v.GetString("data.database.master.driver"),
				Source:          v.GetString("data.database.master.source"),
				Logging:         v.GetBool("data.database.master.logging"),
				MaxIdleConn:     v.GetInt("data.database.master.max_idle_conn"),
				MaxOpenConn:     v.GetInt("data.database.master.max_open_conn"),
				ConnMaxLifeTime: v.GetDuration("data.database.master.max_life_time"),
				Weight:          v.GetInt("data.database.master.weight"),
			},
			Slaves:   getSlaveConfigs(v),
			Migrate:  v.GetBool("data.database.migrate"),
			Strategy: v.GetString("data.database.strategy"),
			MaxRetry: v.GetInt("data.database.max_retry"),
		},
		Redis:         getRedisConfigs(v),
		Meilisearch:   getMeilisearchConfigs(v),
		Elasticsearch: getElasticsearchConfigs(v),
		MongoDB:       getMongoDBConfigs(v),
		Neo4j:         getNeo4jConfigs(v),
		RabbitMQ:      getRabbitMQConfigs(v),
		Kafka:         getKafkaConfigs(v),
	}
}
