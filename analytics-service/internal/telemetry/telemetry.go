package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	otelslog "github.com/go-slog/otelslog"
	slogmulti "github.com/samber/slog-multi"
	otelslog_bridge "go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.28.0"
	"go.opentelemetry.io/otel/trace"
)

var (
    Tracer           trace.Tracer
    Meter            metric.Meter
    MessagesProduced metric.Int64Counter
    MessagesConsumed metric.Int64Counter
)

// InitTelemetry sets up OpenTelemetry and returns a shutdown function.
func InitTelemetry(ctx context.Context, appName string) (shutdown func(context.Context) error, err error) {
    var shutdownFuncs []func(context.Context) error

    // Shutdown handler
    shutdown = func(ctx context.Context) error {
        var err error
        for _, fn := range shutdownFuncs {
            err = errors.Join(err, fn(ctx))
        }
        shutdownFuncs = nil
        return err
    }

    // Handle errors with shutdown
    handleErr := func(inErr error) {
        err = errors.Join(inErr, shutdown(ctx))
    }

    // Set up logger
    handler := otelslog.NewHandler(slog.NewJSONHandler(
        os.Stdout,
        &slog.HandlerOptions{Level: slog.LevelDebug},
    ))
    b_handler := otelslog_bridge.NewHandler(appName)
    logger := slog.New(
        slogmulti.Fanout(
            handler,
            b_handler,
        ),
    )
    slog.SetDefault(logger)
    slog.Info("Initializing telemetry for " + appName)

    // Propagator
    prop := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
    otel.SetTextMapPropagator(prop)

    // Define shared telemetry variables
    var app_resource = resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceNameKey.String(appName),
    )

    // Log exporter    
    logExporter, err := otlploghttp.New(ctx)
    if err != nil {
        slog.Error("Failed to create OTLP log exporter", slog.Any("error", err))
        return nil, err
    }
    logProvider := sdklog.NewLoggerProvider(
        sdklog.WithProcessor(
            sdklog.NewBatchProcessor(logExporter),
        ),
        sdklog.WithResource(app_resource),
    )
    global.SetLoggerProvider(logProvider)
    shutdownFuncs = append(shutdownFuncs, logExporter.Shutdown)
    shutdownFuncs = append(shutdownFuncs, logProvider.Shutdown)
    slog.Info("Log exporter set up for " + appName)

    // Trace exporter
    traceExporter, err := otlptracehttp.New(ctx)
    if err != nil {
        slog.Error("Failed to create OTLP trace exporter", slog.Any("error", err))
        return nil, err
    }
    traceProvider := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(traceExporter),
        sdktrace.WithResource(app_resource),
    )
    otel.SetTracerProvider(traceProvider)
    shutdownFuncs = append(shutdownFuncs, traceProvider.Shutdown)
    Tracer = otel.Tracer(appName)
    slog.Info("Tracer set up for " + appName)

    // Metric exporter
    metricExporter, err := otlpmetrichttp.New(ctx)
    if err != nil {
        slog.Error("Failed to create OTLP metric exporter", slog.Any("error", err))
        return nil, err
    }
    meterProvider := sdkmetric.NewMeterProvider(
        sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
        sdkmetric.WithResource(app_resource),
    )
    otel.SetMeterProvider(meterProvider)
    shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
    Meter = otel.Meter(appName)
    slog.Info("Meter set up for " + appName)

    // Initialize counters
    MessagesProduced, err = Meter.Int64Counter("messages_produced_total", metric.WithDescription("Total messages produced"))
    if err != nil {
        slog.Error("Failed to create messages_produced counter", slog.Any("error", err))
        handleErr(err)
        return
    }
    MessagesConsumed, err = Meter.Int64Counter("messages_consumed_total", metric.WithDescription("Total messages consumed"))
    if err != nil {
        slog.Error("Failed to create messages_consumed counter", slog.Any("error", err))
        handleErr(err)
        return
    }
    slog.Info("Metrics counters initialized")

    // Runtime metrics
    err = runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second))
    if err != nil {
        slog.Error("Failed to start runtime metrics", slog.Any("error", err))
        handleErr(err)
        return
    }

    return shutdown, nil
}