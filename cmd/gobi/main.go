package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/automixer/gobi/producer"
	"github.com/automixer/gobi/promexp"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

const appVersion = "Gobi -- GoFlow2 Prometheus Exporter v.1.0.0"

type Config struct {
	MetricsAddr string
	MetricsPath string
	CreateFifo  bool
}

func main() {
	var (
		cfgFile = flag.String("f", "", "Path to config file")
		ver     = flag.Bool("v", false, "Print version")
		logLvl  = flag.String("ll", "info", "Log level")
		logFmt  = flag.String("lf", "text", "Log format")
	)
	flag.Parse()

	if *ver {
		fmt.Println(appVersion)
		os.Exit(0)
	}

	ll, err := log.ParseLevel(*logLvl)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(ll)
	switch *logFmt {
	case "json":
		log.SetFormatter(&log.JSONFormatter{})
	case "text":
		log.SetFormatter(&log.TextFormatter{})
	default:
		log.Fatal("Log format not available")
	}
	log.Info(fmt.Sprintf("Starting %s", appVersion))

	appConfig := newAppConfig(*cfgFile)
	gProducer, cleanUp := producer.New(appConfig.Producer)
	defer cleanUp()

	promExporters := make([]*promexp.GobiProm, 0, len(appConfig.Promexporters))
	var xNum int
	for _, v := range appConfig.Promexporters {
		pe := promexp.New(v)
		err = gProducer.Register(pe)
		if err != nil {
			log.Error(err)
			continue
		}
		promExporters = append(promExporters, pe)
		xNum++
	}
	log.Info(fmt.Sprintf("%d prometheus exporter(s) registered...", xNum))

	http.Handle(appConfig.Global.MetricsPath, promhttp.Handler())
	log.Fatal(http.ListenAndServe(appConfig.Global.MetricsAddr, nil))
}
