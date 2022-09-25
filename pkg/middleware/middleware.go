package middleware

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const (
	pathAttrKey     = attribute.Key("path")
	methodAttrKey   = attribute.Key("method")
	statusAttrKey   = attribute.Key("status")
	endpointAttrKey = attribute.Key("endpoint")
	typeAttrKey     = attribute.Key("type")
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

func UnaryInterceptor() grpc.UnaryServerInterceptor {
	meter := global.Meter("grpc-server")
	responseTimeHist, err := meter.SyncInt64().Histogram("grpc_response_time",
		instrument.WithDescription("response time"),
		instrument.WithUnit(unit.Milliseconds),
	)
	if err != nil {
		return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
			log.Error().Err(err).Msg("cannot create response time histogram")
			return handler(ctx, req)
		}
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		st := time.Now()

		resp, err = handler(ctx, req)

		responseTimeHist.Record(ctx, time.Since(st).Milliseconds(),
			typeAttrKey.String("unary"),
			endpointAttrKey.String(info.FullMethod),
			statusAttrKey.Int64(int64(status.Code(err))),
		)
		return resp, err
	}
}
