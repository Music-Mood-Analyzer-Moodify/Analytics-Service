package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"analytics-service/internal/telemetry"
	"analytics-service/internal/util"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
)

func SetUpMessaging(connectionString string) {
    conn, err := amqp.Dial(connectionString)
    util.FailOnError(err, "Failed to connect to RabbitMQ")
    defer conn.Close()

    ch, err := conn.Channel()
    util.FailOnError(err, "Failed to open a channel")
    defer ch.Close()

    qCheckSong, err := ch.QueueDeclare(
        "check_song_queue",
        false,
        false,
        false,
        false,
        nil,
    )
    util.FailOnError(err, "Failed to declare check_song_queue")

    qSongPredicted, err := ch.QueueDeclare(
        "song_predicted_queue",
        false,
        false,
        false,
        false,
        nil,
    )
    util.FailOnError(err, "Failed to declare song_predicted_queue")

    msgs, err := ch.Consume(
        qSongPredicted.Name,
        "",
        true,
        false,
        false,
        false,
        nil,
    )
    util.FailOnError(err, "Failed to register a consumer")

    go func() {
        propagator := propagation.TraceContext{}
        for d := range msgs {
            headers := util.RabbitMQHeaderCarrier(d.Headers)
            ctx := propagator.Extract(context.Background(), headers)
            ctx, span := telemetry.Tracer.Start(ctx, "consumeMessage")
            slog.InfoContext(ctx, "Received a message" + string(d.Body))
            span.SetAttributes(attribute.String("message.body", string(d.Body)))
            span.SetAttributes(attribute.String("message.queue", qSongPredicted.Name))
            telemetry.MessagesConsumed.Add(ctx, 1)
            span.End()
        }
    }()

    slog.Info("Waiting for messages. To exit press CTRL+C")
    produceTestMessages(ch, qCheckSong)
}

func produceTestMessages(ch *amqp.Channel, q amqp.Queue) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    idx := 0

    for range ticker.C {
        ctx, span := telemetry.Tracer.Start(context.Background(), "produceMessage")
        body := fmt.Sprintf("Spotify ID: %d", idx)
        err := ch.PublishWithContext(
            ctx,
            "",
            q.Name,
            false,
            false,
            amqp.Publishing{
                ContentType: "text/plain",
                Body:        []byte(body),
            },
        )
        util.FailOnError(err, "Failed to publish a message")
        slog.InfoContext(ctx, "Sent message: " + body)
        span.SetAttributes(attribute.String("message.body", body))
        span.SetAttributes(attribute.String("message.queue", q.Name))
        telemetry.MessagesProduced.Add(ctx, 1)
        span.End()
        idx++
    }
}