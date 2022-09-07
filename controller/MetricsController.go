package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/michibiki-io/hems-metrics-go/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

type MetricsController struct {
	logger                        *zap.Logger
	cumulativePowerConsumption    prometheus.Gauge
	powerConsumptionPerUnitTime   prometheus.Gauge
	instantaneousPowerConsumption prometheus.Gauge
	current                       prometheus.Gauge
	powerFactor                   prometheus.Gauge
}

func CreateMetricsController(l *zap.Logger) *MetricsController {
	c := MetricsController{
		logger: l,
		cumulativePowerConsumption: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "hems",
			Name:      "cumulative_power_consumption",
			Help:      "Cumulative Power Consumption [kWh]",
		}),
		powerConsumptionPerUnitTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "hems",
			Name:      "latest_cumulative_power_consumption_per_unit_time",
			Help:      "Latest Cumulative Power Consumption per Unit time [kWh]",
		}),
		instantaneousPowerConsumption: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "hems",
			Name:      "instantaneous_power_consumption",
			Help:      "Instantaneous Power Consumption [W]",
		}),
		current: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "hems",
			Name:      "current",
			Help:      "Current [A]",
		}),
		powerFactor: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "hems",
			Name:      "power_factor",
			Help:      "Power Factor [%]",
		}),
	}

	prometheus.MustRegister(c.cumulativePowerConsumption,
		c.powerConsumptionPerUnitTime,
		c.instantaneousPowerConsumption,
		c.current,
		c.powerFactor)

	return &c
}

func (controller *MetricsController) Update(model *model.HemsData) {

	if model != nil {
		controller.cumulativePowerConsumption.Set(float64(model.CumulativePowerConsumption))
		controller.powerConsumptionPerUnitTime.Set(float64(model.PowerConsumptionPerUnitTime))
		controller.instantaneousPowerConsumption.Set(float64(model.InstantaneousPowerConsumption))
		controller.current.Set(float64(model.Current))
		controller.powerFactor.Set(float64(model.PowerFactor))
	}
}

func CreatePrometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()

	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}
