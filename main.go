package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	yaml "gopkg.in/yaml.v2"
)

const version string = "0.1"

type List struct {
	Names map[string]string
}

var (
	showVersion   = flag.Bool("version", false, "Print version information.")
	listenAddress = flag.String("listen-address", ":9330", "Address on which to expose metrics.")
	metricsPath   = flag.String("path", "/metrics", "Path under which to expose metrics.")
	ignoreUnknown = flag.Bool("ignore", true, "Ignores sensors without a name")
	nameFile      = flag.String("names", "names.yaml", "File mapping IDs to names")

	list List
)

func init() {
	flag.Usage = func() {
		fmt.Println("Usage: onewire_exporter [ ... ]\n\nParameters:")
		fmt.Println()
		flag.PrintDefaults()
	}
}

func main() {
	promslogConfig := &promslog.Config{}
	flag.StringVar(&promslogConfig.Level, "log.level", "info", "Log level (debug, info, warn, error)")
	flag.StringVar(&promslogConfig.Format, "log.format", "json", "Log format (logfmt, json)")
	flag.Parse()

	logger := promslog.New(promslogConfig)

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	filename, _ := filepath.Abs(*nameFile)
	yamlFile, err := ioutil.ReadFile(filename)

	if err != nil {
		logger.Fatal("Can't read names file")
	}

	err = yaml.Unmarshal(yamlFile, &list)
	if err != nil {
		logger.Fatal("Can't read names file")
	}

	startServer(logger)
}

func printVersion() {
	fmt.Println("onewire_exporter")
	fmt.Printf("Version: %s\n", version)
}

func startServer(logger *slog.Logger) {
	logger.Infof("Starting onewire exporter (Version: %s)\n", version)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>onewire Exporter (Version ` + version + `)</title></head>
			<body>
			<h1>onewire Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			<h2>More information:</h2>
			<p><a href="https://github.com/l3akage/onewire_exporter">github.com/l3akage/onewire_exporter</a></p>
			</body>
			</html>`))
	})
	http.HandleFunc(*metricsPath, handleMetricsRequest)
	http.HandleFunc(*metricsPath, func(w http.ResponseWriter, r *http.Request) {
		handleMetricsRequest(w, r, logger)
	})

	logger.Infof("Listening for %s on %s\n", *metricsPath, *listenAddress)
	logger.Fatal(http.ListenAndServe(*listenAddress, nil))
}

func handleMetricsRequest(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	registry := prometheus.NewRegistry()
	c := &onewireCollector{
		logger: logger,
	}
	c := collector.New(r.Context(), target, authName, snmpContext, auth, nmodules, logger, exporterMetrics, *concurrency, debug)

	registry.MustRegister(c)

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
