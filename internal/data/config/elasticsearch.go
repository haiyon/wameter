package config

import "github.com/spf13/viper"

// Elasticsearch elasticsearch config struct
type Elasticsearch struct {
	Addresses []string `mapstructure:"addresses"`
	Username  string   `mapstructure:"username"`
	Password  string   `mapstructure:"password"`
}

// getElasticsearchConfigs reads Elasticsearch configurations
func getElasticsearchConfigs(v *viper.Viper) *Elasticsearch {
	return &Elasticsearch{
		Addresses: v.GetStringSlice("data.elasticsearch.addresses"),
		Username:  v.GetString("data.elasticsearch.username"),
		Password:  v.GetString("data.elasticsearch.password"),
	}
}
