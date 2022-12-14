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
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/view"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

type Option func(promOption) promOption

type promOption struct {
	port string
}

func newDefaultOption() promOption {
	return promOption{
		port: "2222",
	}
}

func WithOverridePort(port string) Option {
	return func(po promOption) promOption {
		po.port = port
		return po
	}
}

func New(ctx context.Context, opts ...Option) {
	exporter, cancel := createExporter(ctx)
	defer cancel()

	options := newDefaultOption()
	for _, opt := range opts {
		options = opt(options)
	}

	// Start the prometheus HTTP server and pass the exporter Collector to it
	go serveMetrics(exporter.Collector, options)

	ctx, _ = signal.NotifyContext(ctx, os.Interrupt)
	<-ctx.Done()
	log.Info().Msg("stop serving metrics")
}

func serveMetrics(collector prometheus.Collector, po promOption) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector, collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), collectors.NewGoCollector())

	port := fmt.Sprintf(":%s", po.port)
	log.Info().Msg(fmt.Sprintf("serving metrics at localhost%s/metrics", port))
	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	srv := &http.Server{
		Addr: port,
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

	v, err := view.New(view.MatchInstrumentKind(view.SyncHistogram), view.WithSetAggregation(aggregation.ExplicitBucketHistogram{
		Boundaries: []float64{0, 5, 10, 25, 50, 75, 100, 250, 500, 1000, 2000, 5000, 7000, 10000, 100000},
		NoMinMax:   false,
	}))
	if err != nil {
		panic(err)
	}

	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(exporter, v),
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
