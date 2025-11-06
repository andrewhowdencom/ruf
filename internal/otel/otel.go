package otel

import (
	"context"
	"errors"
	"log"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"
)

// SetupOTelSDK bootstraps the OpenTelemetry pipeline.
// If it does not return an error, make sure to call shutdown for proper cleanup.
func SetupOTelSDK(ctx context.Context, traceEndpoint string, traceHeaders map[string]string, metricEndpoint string, metricHeaders map[string]string) (shutdown func(context.Context) error, err error) {
	var shutdownFuncs []func(context.Context) error

	// shutdown calls cleanup functions registered via shutdownFuncs.
	// The errors from the calls are joined.
	// Each registered cleanup will be invoked once.
	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	prop := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(prop)

	if traceEndpoint != "" {
		traceExporter, err := otlptrace.New(ctx, otlptracehttp.NewClient(
			otlptracehttp.WithEndpointURL(traceEndpoint),
			otlptracehttp.WithHeaders(traceHeaders),
		))
		if err != nil {
			return nil, err
		}

		tracerProvider := trace.NewTracerProvider(trace.WithBatcher(traceExporter))
		shutdownFuncs = append(shutdownFuncs, tracerProvider.Shutdown)
		otel.SetTracerProvider(tracerProvider)
	}

	if metricEndpoint != "" {
		metricExporter, err := otlpmetrichttp.New(ctx,
			otlpmetrichttp.WithEndpointURL(metricEndpoint),
			otlpmetrichttp.WithHeaders(metricHeaders),
		)
		if err != nil {
			return nil, err
		}

		meterProvider := metric.NewMeterProvider(metric.WithReader(metric.NewPeriodicReader(metricExporter)))
		shutdownFuncs = append(shutdownFuncs, meterProvider.Shutdown)
		otel.SetMeterProvider(meterProvider)

		err = runtime.Start(runtime.WithMinimumReadMemStatsInterval(time.Second))
		if err != nil {
			log.Fatal(err)
		}
	}

	return
}
