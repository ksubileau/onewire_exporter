package collector

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type Temp struct {
	ID    string
	Value float64
}

type Metrics struct {
	Temperature *prometheus.GaugeVec
	Up          prometheus.Gauge
}

type Collector struct {
	ctx               context.Context
	logger            *slog.Logger
	metrics           Metrics
	ignoreUnknown     bool
	sensorNames       map[string]string
	onewireDevicePath string
}

func New(ctx context.Context, logger *slog.Logger, metrics Metrics, ignoreUnknown bool, sensorNames map[string]string, onewireDevicePath string) *Collector {
	return &Collector{
		ctx:               ctx,
		logger:            logger,
		metrics:           metrics,
		ignoreUnknown:     ignoreUnknown,
		sensorNames:       sensorNames,
		onewireDevicePath: onewireDevicePath,
	}
}

/*
func ObserveOnewireTemperature(logger *slog.Logger, metrics Metrics) {
	// lists onewire devices
	err := onewire.CreateOnewireDeviceList()
	if err != nil {
		log.Fatal("Error getting Onewire device list")
	}
	for {
		sensors = sensors[:len(onewireDeviceList)]
		index := 0
		for _, deviceID := range onewireDeviceList {
			value, err := onewire.ReadOnewireDevicePayload(deviceID)
			//if err != nil {
			//	log.WithFields(log.Fields{"deviceID": deviceID}).Error("Error reading from device")
			//}
			//log.WithFields(log.Fields{"deviceID": deviceID, "value": value, "hostname": hostname}).Info("Value read from device")
			//onewireTemperatureC.With(prometheus.Labels{"device_id": deviceID, "hostname": hostname}).Set(value)
			sensors[index] = sensor{SensorID: deviceID, SensorType: "temperature", SensorValue: value}
			index++
		}
		time.Sleep(60 * time.Second)
	}
}
*/

func getTemperatureFromDevice(onewireDevicePath string, device os.DirEntry, logger *slog.Logger) Temp {
	for i := 1; i <= 5; i++ {
		devicePath := filepath.Join(onewireDevicePath, device.Name(), "w1_slave")
		content, err := os.ReadFile(devicePath)
		if err != nil {
			logger.Info(fmt.Sprintf("Error reading device %s\n", device.Name()))
			continue
		}
		temp, err := parseSensorData(string(content))
		if err != nil {
			logger.Info(fmt.Sprintf("Error reading device %s\n", device.Name()))
			continue
		}

		if temp == 85.0 {
			continue
		}

		return Temp{
			ID:    device.Name(),
			Value: temp,
		}
	}
	return Temp{}
}

func parseSensorData(content string) (float64, error) {
	reg, err := regexp.Compile("[^0-9-]+")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(content, "\n")
	if len(lines) != 3 {
		return 0, fmt.Errorf("Unknown format")
	}
	if !strings.Contains(lines[0], "YES") {
		return 0, fmt.Errorf("CRC invalid")
	}
	data := strings.SplitAfter(lines[1], "t=")
	if len(data) != 2 {
		return 0, fmt.Errorf("Temp value not found")
	}
	strValue := reg.ReplaceAllString(data[1], "")

	temp, err := strconv.ParseFloat(strValue, 64)
	if err != nil {
		return 0, err
	}

	return temp / 1000.0, nil
}

func getTemperatures(logger *slog.Logger, onewireDevicePath string) ([]Temp, error) {
	devices, err := os.ReadDir(onewireDevicePath)
	if err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	valueChan := make(chan Temp)
	for _, device := range devices {
		devicePath := filepath.Join(onewireDevicePath, device.Name(), "w1_slave")
		if _, err := os.Stat(devicePath); err != nil {
			continue
		}
		wg.Add(1)
		go func(device os.DirEntry) {
			defer wg.Done()
			valueChan <- getTemperatureFromDevice(onewireDevicePath, device, logger)
		}(device)
	}
	go func() {
		wg.Wait()
		close(valueChan)
	}()
	var values []Temp
	for t := range valueChan {
		if t == (Temp{}) {
			continue
		}
		values = append(values, t)
	}
	return values, nil
}

func (c Collector) Update() {
	logger := c.logger
	values, err := getTemperatures(c.logger, c.onewireDevicePath)
	if err != nil {
		logger.Error(fmt.Sprintf("Error getting sensor data", err))
		c.metrics.Up.Set(0)
	} else {
		c.metrics.Up.Set(1)
		for _, sensor := range values {
			n := c.sensorNames[sensor.ID]
			if n == "" {
				if c.ignoreUnknown == true {
					c.logger.Info(fmt.Sprintf("Ignoring unknown device %s\n", sensor.ID))
					continue
				} else {
					n = sensor.ID
				}
			}

			c.metrics.Temperature.With(prometheus.Labels{
				"id":   sensor.ID,
				"name": n,
			}).Set(sensor.Value)
		}
	}
}
