package connection

import (
	"context"
	"fmt"
	"wameter/internal/data/config"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// newNeo4j creates a new Neo4j client
func newNeo4j(conf *config.Neo4j) (neo4j.DriverWithContext, error) {
	if conf == nil || conf.URI == "" {
		return nil, fmt.Errorf("neo4j configuration is nil or empty")
	}

	driver, err := neo4j.NewDriverWithContext(conf.URI, neo4j.BasicAuth(conf.Username, conf.Password, ""))
	if err != nil {
		return nil, fmt.Errorf("neo4j connect error: %w", err)
	}

	if err := driver.VerifyConnectivity(context.Background()); err != nil {
		return nil, fmt.Errorf("neo4j verify connectivity error: %w", err)
	}

	return driver, nil
}
