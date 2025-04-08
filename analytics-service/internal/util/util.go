package util

import (
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

// FailOnError logs and panics if an error occurs.
func FailOnError(err error, msg string) {
    if err != nil {
        slog.Error(msg, slog.Any("error", err))
        panic(err)
    }
}

// RabbitMQHeaderCarrier adapts amqp.Table for OpenTelemetry propagation.
type RabbitMQHeaderCarrier amqp.Table

func (c RabbitMQHeaderCarrier) Get(key string) string {
    if val, ok := c[key]; ok {
        if str, ok := val.(string); ok {
            return str
        }
    }
    return ""
}

func (c RabbitMQHeaderCarrier) Set(key string, value string) {
    c[key] = value
}

func (c RabbitMQHeaderCarrier) Keys() []string {
    keys := make([]string, 0, len(c))
    for k := range c {
        keys = append(keys, k)
    }
    return keys
}