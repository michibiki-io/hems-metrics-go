package main

import (
	"context"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/gin-gonic/gin"
	"github.com/michibiki-io/hems-metrics-go/controller"

	"github.com/michibiki-io/goutils"
)

func main() {

	var logger *zap.Logger = nil

	if strings.ToLower(goutils.GetEnv("MODE", "release")) == "debug" {
		logger, _ = zap.NewDevelopment()
	} else {
		logCfg := zap.NewProductionConfig()
		logCfg.EncoderConfig.EncodeTime = func(t time.Time, pae zapcore.PrimitiveArrayEncoder) {
			const layout = "2006-01-02 15:04:05 JST"
			jst := time.FixedZone("Asia/Tokyo", 9*60*60)
			pae.AppendString(t.In(jst).Format(layout))
		}
		logger, _ = logCfg.Build()
	}

	defer logger.Sync()

	// Bルート認証パスワード
	rbpwd := goutils.GetEnv("B_ROUTE_PASSWORD", "0123456789AB")

	// Bルート認証ID
	rbid := goutils.GetEnv("B_ROUTE_ID", "0123456789ABCDEF0123456789ABCDEF")

	// controller
	hemsDataController := controller.CreateHemsDataController(logger)

	// metrics server
	metricsController := controller.CreateMetricsController(logger)

	// set handler
	hemsDataController.RegistHandler(metricsController.Update)

	// context
	ctx := context.Background()

	// Main routine for get meter data
	go func() {
		for {
			if hemsDataController.Initialize(ctx, rbpwd, rbid) == nil {
				hemsDataController.Collect(ctx)
			}
			time.Sleep(time.Duration(5) * time.Second)
		}
	}()

	engine := gin.Default()
	engine.GET("/", func(c *gin.Context) {
		c.JSON(200, "ok")
	})
	engine.GET("/readiness", func(c *gin.Context) {
		if hemsDataController.Readiness() {
			c.JSON(200, "ok")
		} else {
			c.JSON(404, "ng")
		}
	})
	engine.GET("/metrics", controller.CreatePrometheusHandler())
	engine.Run(":9000")
}
