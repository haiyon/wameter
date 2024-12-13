package connection

import (
	"context"
	"fmt"
	"wameter/internal/data/config"

	"github.com/segmentio/kafka-go"
)

// newKafka creates new Kafka connection
func newKafka(cfg *config.Kafka) (*kafka.Conn, error) {
	if cfg == nil || len(cfg.Brokers) == 0 {
		return nil, fmt.Errorf("kafka configuration is nil or empty")
	}

	conn, err := kafka.DialContext(context.Background(), "tcp", cfg.Brokers[0])
	if err != nil {
		return nil, fmt.Errorf("kafka connection error: %w", err)
	}

	return conn, nil
}
