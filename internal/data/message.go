package data

import (
	"context"
	"fmt"
)

var (
	ErrRabbitMQNotInitialized = fmt.Errorf("RabbitMQ service not initialized")
	ErrKafkaNotInitialized    = fmt.Errorf("kafka service not initialized")
)

func (d *Data) PublishToRabbitMQ(exchange, routingKey string, body []byte) error {
	if d.RabbitMQ == nil {
		return ErrRabbitMQNotInitialized
	}
	return d.RabbitMQ.PublishMessage(exchange, routingKey, body)
}

func (d *Data) ConsumeFromRabbitMQ(queue string, handler func([]byte) error) error {
	if d.RabbitMQ == nil {
		return ErrRabbitMQNotInitialized
	}
	return d.RabbitMQ.ConsumeMessages(queue, handler)
}

func (d *Data) PublishToKafka(ctx context.Context, topic string, key, value []byte) error {
	if d.Kafka == nil {
		return ErrKafkaNotInitialized
	}
	return d.Kafka.PublishMessage(ctx, topic, key, value)
}

func (d *Data) ConsumeFromKafka(ctx context.Context, topic, groupID string, handler func([]byte) error) error {
	if d.Kafka == nil {
		return ErrKafkaNotInitialized
	}
	return d.Kafka.ConsumeMessages(ctx, topic, groupID, handler)
}
