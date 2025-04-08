package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"analytics-service/internal/repository"
	"analytics-service/internal/telemetry"
	"analytics-service/internal/util"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
)

func SetUpMessaging(connectionString string) {
    conn, err := amqp.Dial(connectionString)
    util.FailOnError(err, "Failed to connect to RabbitMQ")
    ch, err := conn.Channel()
    util.FailOnError(err, "Failed to open a channel")

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

    go consumeMessages(msgs, qSongPredicted)
    go produceTestMessages(conn, ch, qCheckSong)
}

func consumeMessages(msgs <-chan amqp.Delivery, qSongPredicted amqp.Queue) {
    slog.Info("Waiting for messages.")
    propagator := propagation.TraceContext{}
    for d := range msgs {
        headers := util.RabbitMQHeaderCarrier(d.Headers)
        ctx := propagator.Extract(context.Background(), headers)
        ctx, span := telemetry.Tracer.Start(ctx, "consumeMessage")
        slog.InfoContext(ctx, "Received a message: " + string(d.Body))
        span.SetAttributes(attribute.String("message.body", string(d.Body)))
        span.SetAttributes(attribute.String("message.queue", qSongPredicted.Name))
        telemetry.MessagesConsumed.Add(ctx, 1)

        id, err := repository.CreateMessage(string(d.Body))
        util.FailOnError(err, "Failed to create message in database")
        slog.InfoContext(ctx, "Created message in database with ID: "+fmt.Sprint(id))
        span.SetAttributes(attribute.Int("message.id", id))

        span.End()
    }
}

func produceTestMessages(conn *amqp.Connection, ch *amqp.Channel, q amqp.Queue) {
    slog.Info("Producing test messages every 30 seconds.")
    ticker := time.NewTicker(30 * time.Second)
    idx := 0

    defer ticker.Stop()
    defer conn.Close()
    defer ch.Close()

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