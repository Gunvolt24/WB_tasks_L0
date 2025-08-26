package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// SetupTracing настраивает OTLP/HTTP экспорт, семплинг и глобальные пропагаторы.
// Возвращает функцию корректного завершения провайдера.
func SetupTracing(
	ctx context.Context,
	serviceName, endpoint string,
	sampleRatio float64,
) (func(context.Context) error, error) {
	// Дефолты: endpoint и границы семплинга [0..1].
	if endpoint == "" {
		endpoint = "localhost:4318"
	}
	if sampleRatio < 0 {
		sampleRatio = 0
	}
	if sampleRatio > 1 {
		sampleRatio = 1
	}

	// Экспортёр OTLP/HTTP без TLS.
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	// Провайдер трейсинга: батч-экспорт, семплинг и ресурсы (имя сервиса).
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(sampleRatio)),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(serviceName),
			attribute.String("telemetry.sdk", "opentelemetry"),
		)),
	)

	// Глобальный провайдер и пропагатор (TraceContext + Baggage).
	otel.SetTracerProvider(traceProvider)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{},
		),
	)

	// Возвращаем Shutdown для graceful stop.
	return traceProvider.Shutdown, nil
}
