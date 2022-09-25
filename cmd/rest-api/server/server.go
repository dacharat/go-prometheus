package server

import (
	"context"
	"crypto/rand"
	"math/big"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/dacharat/go-prometheus/pkg/middleware"
	"github.com/dacharat/go-prometheus/pkg/prom"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

func Start() {
	r := gin.Default()
	r.Use(otelgin.Middleware("jack-api"))
	r.Use(middleware.ResponseTimeMetric())

	r.GET("/health", func(gCtx *gin.Context) {
		gCtx.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/test", func(gCtx *gin.Context) {
		nBig, err := rand.Int(rand.Reader, big.NewInt(1200))
		if err != nil {
			gCtx.JSON(http.StatusInternalServerError, gin.H{"message": err})
			return
		}

		n := nBig.Int64()
		if n > 1100 {
			gCtx.JSON(http.StatusBadRequest, gin.H{"message": "random more than 1000"})
			return
		}

		t := time.Duration(n)
		time.Sleep(time.Millisecond * t)
		gCtx.JSON(http.StatusOK, gin.H{"message": "test"})
	})

	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("listen")
		}
	}()

	go func() {
		prom.New(ctx)
	}()

	// Listen for the interrupt signal.
	<-ctx.Done()

	// Restore default behavior on the interrupt signal and notify user of shutdown.
	stop()
	log.Info().Msg("shutting down gracefully, press Ctrl+C again to force")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exiting")
}
