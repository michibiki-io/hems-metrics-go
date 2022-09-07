package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/gorhill/cronexpr"
	"github.com/michibiki-io/goutils"
	"github.com/michibiki-io/hems-metrics-go/dongle"
	"github.com/michibiki-io/hems-metrics-go/model"
	"go.uber.org/zap"
)

var cronUnitTime = goutils.GetEnv("POWER_CONSUMPTION_CRON_EXPR_STRING", "0,30 * * * *")

type HemsDataController struct {
	logger          *zap.Logger
	dongle          *dongle.DongleUtil
	refreshSecond   time.Duration
	previousData    *model.HemsData
	nextCronTime    time.Time
	hemsDataHandler func(model *model.HemsData)
}

func CreateHemsDataController(l *zap.Logger) *HemsDataController {
	return &HemsDataController{
		logger:        l,
		dongle:        dongle.NewDongleUtil(l),
		refreshSecond: time.Duration(goutils.GetIntEnv("REFRESH_SECONDS", 5)) * time.Second,
		previousData:  nil,
		nextCronTime:  time.Now(),
	}
}

func (controller *HemsDataController) Initialize(ctx context.Context, pwd string, rbID string) error {

	// main cancel context
	ictx, cancel := context.WithCancel(ctx)
	defer cancel()

	// init dongle
	if _, err := controller.dongle.Init(ictx, pwd, rbID); err != nil {
		controller.logger.Fatal("init dongle is failed")
		return err
	} else {
		return nil
	}
}

func (controller *HemsDataController) RegistHandler(handler func(model *model.HemsData)) {
	if handler != nil {
		controller.hemsDataHandler = handler
	}
}

func (controller *HemsDataController) Collect(ctx context.Context) error {

	// main cancel context
	ictx, cancel := context.WithCancel(ctx)
	defer cancel()

	// get data routine
	err := controller.doCollect(ictx)

	// disconnect
	controller.dongle.Disconnect()

	return err
}

func (controller *HemsDataController) doCollect(ctx context.Context) error {

	var err error = nil

	ictx, cancel := context.WithCancel(ctx)
	defer cancel()

	// sync channel
	sync := make(chan string, 2)

	// next
	controller.nextCronTime = cronexpr.MustParse(cronUnitTime).Next(time.Now())

	// call one
	cctx, ccancel := controller.fetch(ictx, sync)
	defer ccancel()

	t := time.NewTicker(controller.refreshSecond)
	defer t.Stop()

Default:
	for {
		select {
		case <-sync:
			cctx, ccancel = controller.fetch(ictx, sync)
			defer ccancel()
		case <-t.C:
			sync <- "fetch"
		case <-cctx.Done():
			err = fmt.Errorf("read from dongle is timeout.")
			controller.logger.Error(err.Error())
			break Default
		}
	}

	return err
}

func (controller *HemsDataController) fetch(ctx context.Context, sync chan string) (context.Context, context.CancelFunc) {
	cctx, ccancel := context.WithTimeout(ctx, controller.refreshSecond*2)
	go controller.dongle.Fetch(cctx, controller.HemsDataHandler, sync)
	return cctx, ccancel
}

func (controller *HemsDataController) HemsDataHandler(result *model.HemsData) {
	if controller.previousData == nil {
		controller.previousData = result
	} else if result.DateTime.After(controller.nextCronTime) {
		powerConsumptionPerUnitTime := result.CumulativePowerConsumption -
			controller.previousData.CumulativePowerConsumption
		result.PowerConsumptionPerUnitTime = powerConsumptionPerUnitTime
		controller.previousData = result
		controller.nextCronTime = cronexpr.MustParse(cronUnitTime).Next(result.DateTime)
	} else {
		result.PowerConsumptionPerUnitTime =
			controller.previousData.PowerConsumptionPerUnitTime
	}
	controller.logger.Debug(fmt.Sprintf("WH: %v [kWh]", result.CumulativePowerConsumption))
	controller.logger.Debug(fmt.Sprintf("W: %v [W]", result.InstantaneousPowerConsumption))
	controller.logger.Debug(fmt.Sprintf("A: %v [A]", result.Current))
	controller.logger.Debug(fmt.Sprintf("PF: %v [%%]", result.PowerFactor))
	controller.logger.Debug(fmt.Sprintf("WH(last 30min): %v [kwh]", result.PowerConsumptionPerUnitTime))

	if controller.hemsDataHandler != nil {
		controller.hemsDataHandler(result)
	}
}
