package data

import (
	"wameter/internal/data"
	"wameter/internal/data/config"
)

// Data represents data implementation
type Data struct {
	*data.Data
}

// New creates new data
func New(conf *config.Config) (*Data, func(name ...string), error) {
	d, cleanup, err := data.New(conf)
	if err != nil {
		return nil, nil, err
	}

	return &Data{
		Data: d,
	}, cleanup, nil
}
