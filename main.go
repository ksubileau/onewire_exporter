package main

import (
	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
	yaml "gopkg.in/yaml.v2"
	"ksubileau/onewire_exporter/collector"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Names map[string]string
}

const (
	namespace = "onewire"
)

var (
	onewireDevicePath = kingpin.Flag("devices.path", "Path under which to find sensors.").Default("/sys/bus/w1/devices/").String()
	metricsPath       = kingpin.Flag("path", "Path under which to expose metrics.").Default("/metrics").String()
	ignoreUnknown     = kingpin.Flag("ignore", "Ignores sensors without a name").Default("true").Bool()
	nameFile          = kingpin.Flag("names", "File mapping IDs to names").Default("names.yaml").String()
	toolkitFlags      = webflag.AddFlags(kingpin.CommandLine, ":9330")

	config Config
)

func main() {
	promslogConfig := &promslog.Config{}
	flag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.Version(version.Print("onewire_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promslog.New(promslogConfig)

	logger.Info("Starting onewire exporter", "version", version.Info())
	logger.Info("operational information", "build_context", version.BuildContext())

	prometheus.MustRegister(versioncollector.NewCollector("onewire_exporter"))

	filename, _ := filepath.Abs(*nameFile)
	yamlFile, err := os.ReadFile(filename)

	if err != nil {
		logger.Error("Can't read configuration file")
		os.Exit(1)
	}

	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		logger.Error("Can't read configuration file")
		os.Exit(1)
	}

	exporterMetrics := collector.Metrics{
		Temperature: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "temperature_celsius",
				Help:      "Sensor temperature (in degrees C).",
			},
			[]string{"id", "name"},
		),
		Up: promauto.NewGauge(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "up",
				Help:      "Scrape was successful.",
			},
		),
	}

	http.Handle(*metricsPath, promhttp.Handler())

	if *metricsPath != "/" && *metricsPath != "" {
		landingConfig := web.LandingConfig{
			Name:        "Onewire Exporter",
			Description: "Prometheus Exporter for one-wire temperature sensors",
			Version:     version.Info(),
			Links: []web.LandingLinks{
				{
					Address: *metricsPath,
					Text:    "Metrics",
				},
			},
		}
		landingPage, err2 := web.NewLandingPage(landingConfig)
		if err2 != nil {
			logger.Error("Error creating landing page", "err", err2)
			os.Exit(1)
		}
		http.Handle("/", landingPage)
	}

	c := collector.New(nil, logger, exporterMetrics, *ignoreUnknown, config.Names, *onewireDevicePath)
	//go collector.ObserveOnewireTemperature(logger, exporterMetrics)
	go func() {
		for {
			c.Update()
			time.Sleep(10 * time.Second)
		}
	}()

	srv := &http.Server{}
	if err3 := web.ListenAndServe(srv, toolkitFlags, logger); err3 != nil {
		logger.Error("Error starting HTTP server", "err", err3)
		os.Exit(1)
	}
}
