package dongle

import (
	"bufio"
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/michibiki-io/goutils"
	"github.com/michibiki-io/hems-metrics-go/utility/constant"
	"github.com/tarm/serial"
	"go.uber.org/zap"
)

func NewDongle(logger *zap.Logger) *Dongle {
	d := &Dongle{
		logger:   logger,
		Baudrate: 115200,
	}

	switch runtime.GOOS {
	case "darwin":
		// mac
		d.SerialDevice = "/dev/tty.usbserial-A103BTKQ"
	default:
		// raspberry pi.
		d.SerialDevice = "/dev/ttyUSB0"
	}

	return d
}

type Dongle struct {
	Baudrate     int
	SerialDevice string
	Port         *serial.Port
	logger       *zap.Logger
}

func (b *Dongle) Connect() error {
	c := &serial.Config{
		Name:        b.SerialDevice,
		Baud:        b.Baudrate,
		ReadTimeout: time.Duration(goutils.GetIntEnv("REFRESH_SECONDS", 5) * 2),
	}
	s, err := serial.OpenPort(c)
	if err != nil {
		return err
	}
	b.Port = s
	return nil
}

func (b *Dongle) Close() {
	b.Port.Close()
}

func (b *Dongle) SKVER() (string, error) {
	err := b.write("SKVER\r\n")
	if err != nil {
		return "", err
	}
	lines, err := b.readUntilOK()
	if err != nil {
		return "", err
	}
	if len(lines) > 1 && strings.Contains(lines[1], " ") {
		return strings.Split(lines[1], " ")[1], nil
	} else {
		return "", fmt.Errorf("bad data response")
	}
}

func (b *Dongle) write(s string) error {
	_, err := b.Port.Write([]byte(s))
	if err != nil {
		return err
	}
	return nil
}

func (b *Dongle) flush() error {
	err := b.Port.Flush()
	if err != nil {
		return err
	}
	return nil
}

func (b *Dongle) readUntilOK() ([]string, error) {
	reader := bufio.NewReader(b.Port)
	scanner := bufio.NewScanner(reader)
	var reply []string
	for scanner.Scan() {
		l := scanner.Text()
		reply = append(reply, l)
		if l == "OK" {
			break
		}
	}
	return reply, nil
}

func (b *Dongle) SKSETPWD(pwd string) error {
	err := b.write("SKSETPWD C " + pwd + "\r\n")
	if err != nil {
		return err
	}
	return nil

}

func (b *Dongle) SKSETRBID(rbid string) error {
	err := b.write("SKSETRBID " + rbid + "\r\n")
	if err != nil {
		return err
	}
	return nil
}

type PAN struct {
	Channel     string
	ChannelPage string
	PanID       string
	Addr        string
	LQI         string
	PairID      string
}

func (b *Dongle) SKSCAN(ctx context.Context, duration int) (*PAN, error) {
	if duration < constant.MinimumSkscanDurationSeoncds {
		duration = constant.MinimumSkscanDurationSeoncds
	}
	err := b.write(fmt.Sprintf("SKSCAN 2 FFFFFFFF %d\r\n", duration))
	if err != nil {
		return nil, err
	}
	scanCh := make(chan PAN)
	scanning := func(ctx context.Context) {
		innerCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		for {
			pan := PAN{}
			reader := bufio.NewReader(b.Port)
			scanner := bufio.NewScanner(reader)
			for scanner.Scan() {
				l := scanner.Text()
				switch {
				case strings.Contains(l, "Channel:"):
					pan.Channel = strings.Split(l, ":")[1]
				case strings.Contains(l, "Channel Page:"):
					pan.ChannelPage = strings.Split(l, ":")[1]
				case strings.Contains(l, "Pan ID:"):
					pan.PanID = strings.Split(l, ":")[1]
				case strings.Contains(l, "Addr:"):
					pan.Addr = strings.Split(l, ":")[1]
				case strings.Contains(l, "LQI:"):
					pan.LQI = strings.Split(l, ":")[1]
				case strings.Contains(l, "PairID:"):
					pan.PairID = strings.Split(l, ":")[1]
				}
				if strings.Contains(l, "EVENT 22 ") {
					break
				}
			}
			select {
			case <-innerCtx.Done():
				return
			default:
				if len(pan.Addr) == 0 || len(pan.Channel) == 0 || len(pan.ChannelPage) == 0 ||
					len(pan.LQI) == 0 || len(pan.PairID) == 0 || len(pan.PanID) == 0 {
					err = fmt.Errorf("PAN data is invalid")
				} else {
					scanCh <- pan
					return
				}
				time.Sleep(time.Second / 2)
			}
		}
	}

	skscanCtx, cancel := context.WithTimeout(ctx, time.Duration(constant.ScanTimeoutSecond)*time.Second)
	defer cancel()
	go scanning(skscanCtx)

	select {
	case res := <-scanCh:
		return &res, nil
	case <-skscanCtx.Done():
		return nil, fmt.Errorf("SKSCAN is timeout")
	}
}

func (b *Dongle) SKSREG(k, v string) error {
	err := b.write("SKSREG " + k + " " + v + "\r\n")
	if err != nil {
		return err
	}
	_, err = b.readUntilOK()
	if err != nil {
		return err
	}
	return nil
}

func (b *Dongle) SKLL64(addr string) (string, error) {
	err := b.write("SKLL64 " + addr + "\r\n")
	if err != nil {
		return "", err
	}
	reader := bufio.NewReader(b.Port)
	r, _, err := reader.ReadLine()
	if err != nil {
		return "", err
	}
	b.logger.Debug(string(r))
	r, _, err = reader.ReadLine()
	if err != nil {
		return "", err
	}
	b.logger.Debug(string(r))
	return string(r), nil
}

func (b *Dongle) SKJOIN(ipv6Addr string) error {
	err := b.write("SKJOIN " + ipv6Addr + "\r\n")
	if err != nil {
		return err
	}
	reader := bufio.NewReader(b.Port)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		l := scanner.Text()
		b.logger.Debug(l)
		if strings.Contains(l, "FAIL ") {
			return fmt.Errorf("Failed to SKJOIN. %s", l)
		}
		if strings.Contains(l, "EVENT 25 ") {
			break
		}
	}
	if scanner.Scan() {
		b.logger.Debug(scanner.Text())
	}
	return nil
}

func (b *Dongle) SKSENDTO(handle, ipAddr, port, sec string, data []byte) (string, error) {
	s := fmt.Sprintf("SKSENDTO %s %s %s %s %.4X ", handle, ipAddr, port, sec, len(data))
	d := append([]byte(s), data[:]...)
	d = append(d, []byte("\r\n")[:]...)
	_, err := b.Port.Write(d)
	if err != nil {
		return "", err
	}
	reader := bufio.NewReader(b.Port)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		l := scanner.Text()
		if l != "" {
			b.logger.Debug("[RESPONSE] >> " + l)
		}
		if strings.Contains(l, "FAIL ") {
			return "", fmt.Errorf("Failed to SKSENDTO. %s", l)
		}
		if strings.Contains(l, "ERXUDP ") {
			return l, nil
		}
	}
	return "", nil
}
