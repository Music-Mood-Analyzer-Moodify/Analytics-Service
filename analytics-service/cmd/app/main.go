package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"

	"analytics-service/internal/controller"
	"analytics-service/internal/messaging"
	"analytics-service/internal/repository"
	"analytics-service/internal/telemetry"
	"analytics-service/internal/util"
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

    db_err := repository.InitTables()
    util.FailOnError(db_err, "Failed to initialize database tables")
    slog.Info("Database tables initialized")

    mux := http.NewServeMux()
    mux.HandleFunc("/messages", controller.GetMessages())

    messaging.SetUpMessaging(rabbitMQConnectionString)

    handler := http.Handler(mux)
    if err := http.ListenAndServe(":8080", handler); err != nil {
        slog.Error("Server failed", "error", err)
    }

    <-ctx.Done()
    slog.Info("Shutting down due to interrupt")
    return nil
}