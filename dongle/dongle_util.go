package dongle

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/michibiki-io/goutils"
	"github.com/michibiki-io/hems-metrics-go/model"
	"github.com/michibiki-io/hems-metrics-go/utility/constant"
	"go.uber.org/zap"
)

func NewDongleUtil(l *zap.Logger) *DongleUtil {
	return &DongleUtil{
		logger: l,
	}
}

type DongleUtil struct {
	logger   *zap.Logger
	dongle   *Dongle
	ipv6addr string
}

func (du *DongleUtil) Init(ctx context.Context, pwd string, rbID string) (bool, error) {

	// dongle init retry count
	connectRetryCount := goutils.GetIntEnv("CONNECT_RETRY_COUNT", 5)

	// init result
	result := false
	var err error = nil

	for counter := 0; counter < connectRetryCount; counter++ {
		err = du.doInit(ctx, pwd, rbID, constant.MinimumSkscanDurationSeoncds+counter)
		if err == nil {
			result = true
			break
		}
	}

	return result, err
}

func (du *DongleUtil) doInit(ctx context.Context, pwd string, rbID string, duration int) error {

	d := NewDongle(du.logger)
	du.dongle = d       // TODO
	logger := du.logger // TODO

	logger.Info("Connect...")
	d.Connect()
	logger.Info("Connect OK.")
	//defer d.Close()

	logger.Debug("Wait 1sec...")
	time.Sleep(time.Second * 1)
	logger.Debug("Wait complete.")

	logger.Debug("SKVER...")
	v, err := d.SKVER()
	logger.Debug(fmt.Sprintf("SKVER Response : %s", v))
	if err != nil {
		logger.Error("SKVER is failed")
		return err
	}
	logger.Info("SKVER OK.")

	err = d.SKSETPWD(pwd)
	if err != nil {
		logger.Error("SKSETPWD is failed")
		return err
	}

	err = d.SKSETRBID(rbID)
	if err != nil {
		logger.Error("SKSETRBID is failed")
		return err
	}

	logger.Debug("SKSCAN...")
	pan, err := d.SKSCAN(ctx, duration)
	logger.Debug(fmt.Sprintf("%#v\n", pan))
	if err != nil {
		logger.Error("SKSCAN is failed")
		return err
	}

	err = d.SKSREG("S2", pan.Channel)
	if err != nil {
		logger.Error("SKSREG S2 is failed")
		return err
	}

	logger.Debug("Set PanID to S3 register...")
	err = d.SKSREG("S3", pan.PanID)
	if err != nil {
		logger.Error("SKSREG S3 is failed")
		return err
	}
	logger.Debug("Get IPv6 Addr with SKLL64...")
	ipv6Addr, err := d.SKLL64(pan.Addr)
	du.ipv6addr = ipv6Addr // TODO
	if err != nil {
		logger.Error("get IPv6 Address is failed")
		return err
	}

	logger.Debug("IPv6 Addr is " + ipv6Addr)
	logger.Debug("SKJOIN...")
	err = d.SKJOIN(ipv6Addr)
	if err != nil {
		logger.Error("SKJOIN is failed")
		return err
	}

	return nil
}

func (du *DongleUtil) Disconnect() {

	du.dongle.Close()

}

var b = []byte{0x10, 0x81, 0x00, 0x01, 0x05, 0xFF, 0x01, 0x02, 0x88, 0x01, 0x62, 0x05, 0xE1, 0x00, 0xE0, 0x00, 0xD7, 0x00, 0xE7, 0x00, 0xE8, 0x00}

func (du *DongleUtil) Fetch(ctx context.Context, f func(result *model.HemsData), queue chan string) error {

	logger := du.logger // TODO

	logger.Debug("SKSENDTO...")
	r, err := du.dongle.SKSENDTO("1", du.ipv6addr, "0E1A", "1", b)
	if err != nil {
		logger.Error("error", zap.Any("err", err))
		f(nil)
		return err
	}
	a := strings.Split(r, " ")
	if len(a) != 9 {
		errmsg := fmt.Sprintf("data length is invalid: %d", len(a))
		logger.Warn(errmsg)
		f(nil)
		return fmt.Errorf(errmsg)
	}
	if a[7] != "0024" {
		logger.Warn(fmt.Sprintf("%s is not 0024. invalid data ? :", a[7]))
		f(nil)
		return nil
	}
	res := a[8]

	// check data
	seoj := res[8 : 8+6]
	ESV := res[20 : 20+2]

	if seoj == "028801" && ESV == "72" {

		sigdigit := 0
		unitnum := float32(1.0)
		pos := 24
		cumulative_power_consumption_base := 0
		cumulative_power_consumption := float32(0.0)
		instantaneous_power_consumption := 0
		instantaneous_current_r_phase := 0
		instantaneous_current_t_phase := 0

		for pos < len(res) {

			// epc
			epc := res[pos : pos+2]

			// pdc
			pdc := res[pos+2 : pos+4]
			datalen, err := strconv.ParseUint(pdc, 16, 0)
			if err != nil {
				continue
			}

			// edt
			edt := res[pos+4 : pos+4+int(datalen)*2]

			// log
			logger.Debug(fmt.Sprintf("%s / %s / %s", epc, pdc, edt))

			pos = pos + 4 + int(datalen)*2

			if epc == "D7" {
				// D7 = 有効桁数
				if tmp, err := strconv.ParseInt(edt, 16, 0); err != nil {
					logger.Warn(fmt.Sprintf("data %s is invalid: %s", epc, edt))
				} else {
					sigdigit = int(tmp)
				}

			} else if epc == "E1" {
				// E1 = 単位
				if edt == "00" {
					unitnum = 1.0
				} else if edt == "01" {
					unitnum = 0.1
				} else if edt == "02" {
					unitnum = 0.01
				} else if edt == "03" {
					unitnum = 0.001
				} else if edt == "04" {
					unitnum = 0.0001
				} else if edt == "0A" {
					unitnum = 10.0
				} else if edt == "0B" {
					unitnum = 100.0
				} else if edt == "0C" {
					unitnum = 1000.0
				} else if edt == "0D" {
					unitnum = 10000.0
				} else {
					logger.Warn(fmt.Sprintf("data %s is invalid: %s", epc, edt))
				}
			} else if epc == "E0" {
				// E0 = 積算電力
				if tmp, err := strconv.ParseInt(edt, 16, 0); err != nil {
					logger.Warn(fmt.Sprintf("data %s is invalid: %s", epc, edt))
				} else {
					cumulative_power_consumption_base = int(tmp)
				}

			} else if epc == "E7" {
				// E7 = 瞬間消費電力
				if tmp, err := strconv.ParseInt(edt, 16, 0); err != nil {
					logger.Warn(fmt.Sprintf("data %s is invalid: %s", epc, edt))
				} else {
					instantaneous_power_consumption = int(tmp)
				}
			} else if epc == "E8" {
				// E8 = 瞬間消費電流
				if tmp, err := strconv.ParseInt(edt[0:len(edt)/2], 16, 16); err != nil {
					logger.Warn(fmt.Sprintf("data %s is invalid: %s", epc, edt[0:len(edt)/2]))
				} else {
					instantaneous_current_r_phase = int(tmp)
				}
				if tmp, err := strconv.ParseInt(edt[len(edt)/2:], 16, 16); err != nil {
					logger.Warn(fmt.Sprintf("data %s is invalid: %s", epc, edt[len(edt)/2:]))
				} else {
					instantaneous_current_t_phase = int(tmp)
				}
			}
		}

		cumulative_power_consumption = float32(cumulative_power_consumption_base) * unitnum

		// str_cumulative_power_consumption := fmt.Sprintf("%f", cumulative_power_consumption)
		// if unitnum < 1 {
		// 	if tmp, err := strconv.ParseFloat(str_cumulative_power_consumption[0:sigdigit+1], 0); err == nil {
		// 		cumulative_power_consumption = float32(tmp)
		// 	}
		// }

		// result structure
		result := model.CreateHemsData(time.Now(),
			cumulative_power_consumption,
			instantaneous_power_consumption,
			instantaneous_current_r_phase, instantaneous_current_t_phase)

		logger.Debug(fmt.Sprintf("sigdigit: %v", sigdigit))
		logger.Debug(fmt.Sprintf("WH: %v [kWh]", result.CumulativePowerConsumption))
		logger.Debug(fmt.Sprintf("W: %v [W]", result.InstantaneousPowerConsumption))
		logger.Debug(fmt.Sprintf("A: %v [A], R phase: %v [A], T phase: %v [A]", result.Current, result.RphaseCurrent, result.TpahseCurrent))
		logger.Debug(fmt.Sprintf("PF: %v [%%]", result.PowerFactor))

		select {
		case <-ctx.Done():
			f(nil)
			return nil
		default:
			f(result) // output
		}

	} else {
		logger.Warn(fmt.Sprintf("data is invalid, seoj:%v, ESV:%v", seoj, ESV))
		f(nil)
		return nil
	}

	return nil
}
