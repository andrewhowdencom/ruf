package otel

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// ShutdownFunc is a function that shuts down the OpenTelemetry providers.
type ShutdownFunc func(context.Context) error

// Init initializes the OpenTelemetry providers and returns a shutdown function.
func Init() (ShutdownFunc, error) {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("ruf"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Set up the OTLP/gRPC trace exporter
	traceExporter, err := newTraceExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Set up the TracerProvider
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	// Set up the OTLP/gRPC metric exporter
	metricExporter, err := newMetricExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// Set up the MeterProvider
	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return func(ctx context.Context) error {
		slog.Info("shutting down opentelemetry providers")
		if err := tracerProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown tracer provider: %w", err)
		}
		if err := meterProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown meter provider: %w", err)
		}
		return nil
	}, nil
}

func newTraceExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	endpoint := viper.GetString("otel.exporter.otlp.endpoint")
	if endpoint == "" {
		slog.Debug("opentelemetry trace exporter disabled")
		return nil, nil
	}
	slog.Info("initialising opentelemetry trace exporter", "endpoint", endpoint)

	var opts []otlptracegrpc.Option
	opts = append(opts, otlptracegrpc.WithEndpoint(endpoint))
	opts = append(opts, otlptracegrpc.WithHeaders(viper.GetStringMapString("otel.exporter.otlp.headers")))
	opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithBlock()))
	if viper.GetBool("otel.exporter.otlp.insecure") {
		opts = append(opts, otlptracegrpc.WithInsecure())
	} else {
		opts = append(opts, otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")))
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}
	return exporter, nil
}

func newMetricExporter(ctx context.Context) (metric.Exporter, error) {
	endpoint := viper.GetString("otel.exporter.otlp.endpoint")
	if endpoint == "" {
		slog.Debug("opentelemetry metric exporter disabled")
		return nil, nil
	}
	slog.Info("initialising opentelemetry metric exporter", "endpoint", endpoint)

	var opts []otlpmetricgrpc.Option
	opts = append(opts, otlpmetricgrpc.WithEndpoint(endpoint))
	opts = append(opts, otlpmetricgrpc.WithHeaders(viper.GetStringMapString("otel.exporter.otlp.headers")))
	opts = append(opts, otlpmetricgrpc.WithDialOption(grpc.WithBlock()))
	if viper.GetBool("otel.exporter.otlp.insecure") {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	} else {
		opts = append(opts, otlpmetricgrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")))
	}

	exporter, err := otlpmetricgrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}
	return exporter, nil
}
