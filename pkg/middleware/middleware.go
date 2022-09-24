package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
)

var (
	pathAttrKey   attribute.Key = "path"
	methodAttrKey attribute.Key = "method"
	statusAttrKey attribute.Key = "status"
)

func ResponseTimeMetric() gin.HandlerFunc {
	meter := global.Meter("gin-server")
	responseTimeHistogram, err := meter.SyncInt64().Histogram(
		"http_response_time",
		instrument.WithDescription("response time"),
		instrument.WithUnit(unit.Milliseconds),
	)
	if err != nil {
		log.Error().Err(err).Msg("cannot create response time histogram")
		return func(gCtx *gin.Context) {
			gCtx.Next()
		}
	}

	return func(gCtx *gin.Context) {
		path := gCtx.FullPath()
		st := time.Now()

		gCtx.Next()

		responseTimeHistogram.Record(gCtx,
			time.Since(st).Milliseconds(),
			pathAttrKey.String(path),
			methodAttrKey.String(gCtx.Request.Method),
			statusAttrKey.Int(gCtx.Writer.Status()))
	}
}
