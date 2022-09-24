package prom

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func New(ctx context.Context) {
	exporter, cancel := createExporter(ctx)
	defer cancel()
	// Start the prometheus HTTP server and pass the exporter Collector to it
	go serveMetrics(exporter.Collector)

	ctx, _ = signal.NotifyContext(ctx, os.Interrupt)
	<-ctx.Done()
}

func serveMetrics(collector prometheus.Collector) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector, collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), collectors.NewGoCollector())

	log.Info().Msg("serving metrics at localhost:2222/metrics")
	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	srv := &http.Server{
		Addr: ":2222",
	}

	err := srv.ListenAndServe()
	if err != nil {
		fmt.Printf("error serving http: %v", err)
		return
	}
}

func createExporter(ctx context.Context) (otelprom.Exporter, func()) {
	// See the go.opentelemetry.io/otel/sdk/resource package for more
	// information about how to create and use Resources.
	exporter := otelprom.New()

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String("my-service"),
		semconv.ServiceVersionKey.String("v0.1.0"),
	)

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(exporter),
	)
	global.SetMeterProvider(meterProvider)

	cancel := func() {
		log.Info().Msg("shutdown meter")
		err := meterProvider.Shutdown(ctx)
		if err != nil {
			log.Fatal().Err(err)
		}
	}

	return exporter, cancel
}
