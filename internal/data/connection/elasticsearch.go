package connection

import (
	"fmt"
	"io"
	"wameter/internal/data/config"
	"wameter/internal/data/elastic"
)

// newElasticsearch creates new Elasticsearch client
func newElasticsearch(cfg *config.Elasticsearch) (*elastic.Client, error) {
	if cfg == nil || len(cfg.Addresses) == 0 {
		return nil, fmt.Errorf("elasticsearch configuration is nil or empty")
	}

	es, err := elastic.NewClient(cfg.Addresses, cfg.Username, cfg.Password)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch client creation error: %w", err)
	}

	res, err := es.GetClient().Info()
	if err != nil {
		return nil, fmt.Errorf("elasticsearch connect error: %w", err)
	}

	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(res.Body)

	if res.IsError() {
		return nil, fmt.Errorf("elasticsearch info error: %s", res.Status())
	}

	return es, nil
}
