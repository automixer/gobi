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

var (
	appName    = ""
	appVersion = ""
	buildDate  = ""
)

func main() {
	var (
		cfgFile = flag.String("f", "", "Path to config file")
		ver     = flag.Bool("v", false, "Print version")
		logLvl  = flag.String("ll", "info", "Log level")
		logFmt  = flag.String("lf", "text", "Log format")
	)
	flag.Parse()

	if *ver {
		fmt.Println("Gobi -- Flows Monitoring Tool --")
		fmt.Println("Release:", appVersion)
		fmt.Println("Build date:", buildDate)
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
	log.Info(fmt.Sprintf("Starting %s %s", appName, appVersion))

	appConfig := newAppConfig(*cfgFile)
	gProducer, cleanUp := producer.New(appConfig.Producer)
	defer cleanUp()

	var xNum int
	for _, v := range appConfig.Promexporters {
		pe := promexp.New(v)
		err = gProducer.Register(pe)
		if err != nil {
			log.Error(err)
			continue
		}
		xNum++
	}
	log.Info(fmt.Sprintf("%d prometheus exporter(s) registered...", xNum))

	http.Handle(appConfig.Global.MetricsPath, promhttp.Handler())
	log.Fatal(http.ListenAndServe(appConfig.Global.MetricsAddr, nil))
}
