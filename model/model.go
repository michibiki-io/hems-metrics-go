package model

import (
	"math"
	"time"
)

type HemsData struct {
	DateTime                      time.Time
	CumulativePowerConsumption    float32
	PowerConsumptionPerUnitTime   float32
	InstantaneousPowerConsumption int
	Current                       float32
	RphaseCurrent                 float32
	TpahseCurrent                 float32
	PowerFactor                   float32
}

func CreateHemsData(
	dateTime time.Time, cpc float32, ipc int,
	rCurrent int, tCurrent int) *HemsData {

	return &HemsData{
		DateTime:                      dateTime,
		CumulativePowerConsumption:    cpc,
		InstantaneousPowerConsumption: ipc,
		Current:                       float32(rCurrent+tCurrent) * 0.1,
		RphaseCurrent:                 float32(rCurrent) * 0.1,
		TpahseCurrent:                 float32(tCurrent) * 0.1,
		PowerFactor:                   float32(math.Round(float64(ipc)*1000.0/(10.0*float64(rCurrent+tCurrent))) * 0.1),
	}
}
