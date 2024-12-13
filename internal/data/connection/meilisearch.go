package connection

import (
	"fmt"
	"wameter/internal/data/config"
	"wameter/internal/data/meili"
)

// newMeilisearch creates new Meilisearch client
func newMeilisearch(cfg *config.Meilisearch) (*meili.Client, error) {
	if cfg == nil || cfg.Host == "" {
		return nil, fmt.Errorf("meilisearch configuration is nil or empty")
	}

	ms := meili.New(cfg.Host, cfg.APIKey)

	if _, err := ms.GetClient().Health(); err != nil {
		return nil, fmt.Errorf("meilisearch connection error: %w", err)
	}

	return ms, nil
}
