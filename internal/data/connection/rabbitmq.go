package connection

import (
	"context"
	"fmt"
	"wameter/internal/data/config"

	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/appengine/log"
)

// newRabbitMQ creates new RabbitMQ connection
func newRabbitMQ(cfg *config.RabbitMQ) (*amqp.Connection, error) {
	if cfg == nil || cfg.URL == "" {
		log.Infof(context.Background(), "RabbitMQ configuration is nil or empty")
		return nil, nil
	}

	url := fmt.Sprintf("amqp://%s:%s@%s/%s", cfg.Username, cfg.Password, cfg.URL, cfg.Vhost)
	conn, err := amqp.DialConfig(url, amqp.Config{
		Heartbeat: cfg.HeartbeatInterval,
		Vhost:     cfg.Vhost,
	})
	if err != nil {
		log.Errorf(context.Background(), "Failed to connect to RabbitMQ: %v", err)
		return nil, err
	}

	log.Infof(context.Background(), "Connected to RabbitMQ")
	return conn, nil
}
