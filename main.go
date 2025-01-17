package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promslog"
	yaml "gopkg.in/yaml.v2"
)

const version string = "0.2"

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
	flag.Parse()

	logger := promslog.New(promslogConfig)

	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	filename, _ := filepath.Abs(*nameFile)
	yamlFile, err := os.ReadFile(filename)

	if err != nil {
		logger.Error("Can't read names file")
		os.Exit(1)
	}

	err = yaml.Unmarshal(yamlFile, &list)
	if err != nil {
		logger.Error("Can't read names file")
		os.Exit(1)
	}

	startServer(logger)
}

func printVersion() {
	fmt.Println("onewire_exporter")
	fmt.Printf("Version: %s\n", version)
}

func startServer(logger *slog.Logger) {
	logger.Info(fmt.Sprintf("Starting onewire exporter (Version: %s)\n", version))
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

	http.HandleFunc(*metricsPath, func(w http.ResponseWriter, r *http.Request) {
		handleMetricsRequest(w, r, logger)
	})

	logger.Info(fmt.Sprintf("Listening for %s on %s\n", *metricsPath, *listenAddress))

	if err := http.ListenAndServe(*listenAddress, nil); err != nil {
		logger.Error("Error starting HTTP server", "err", err)
		os.Exit(1)
	}
}

func handleMetricsRequest(w http.ResponseWriter, r *http.Request, logger *slog.Logger) {
	registry := prometheus.NewRegistry()
	c := &onewireCollector{
		logger: logger,
	}

	registry.MustRegister(c)

	h := promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
	h.ServeHTTP(w, r)
}
