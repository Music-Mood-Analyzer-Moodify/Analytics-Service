package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"analytics-service/internal/messaging"
	"analytics-service/internal/telemetry"
)

func main() {
    if err := run(); err != nil {
        println("Error:", err)
    }
}

func run() error {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
    defer stop()

    appName := os.Getenv("APP_NAME")
    rabbitMQConnectionString := os.Getenv("RABBITMQ_CONNECTION_STRING")

    telemetryShutdown, err := telemetry.InitTelemetry(ctx, appName)
    if err != nil {
        return err
    }
    defer func() {
        if shutdownErr := telemetryShutdown(context.Background()); shutdownErr != nil {
            slog.Error("Telemetry shutdown failed", slog.Any("error", shutdownErr))
        }
    }()

    slog.Info("Starting Analytics Service")
    messaging.SetUpMessaging(rabbitMQConnectionString)

    <-ctx.Done()
    slog.Info("Shutting down due to interrupt")
    return nil
}