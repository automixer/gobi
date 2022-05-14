package main

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/automixer/gobi/producer"
	"github.com/automixer/gobi/promexp"
	log "github.com/sirupsen/logrus"
)

type appConfig struct {
	Global        Config
	Producer      producer.Config
	Promexporters []promexp.Config
}

type Config struct {
	MetricsAddr string
	MetricsPath string
	CreateFifo  bool
}

func newAppConfig(fName string) appConfig {
	cfg := appConfig{}

	yCfg := loadCfgFromFile(fName)
	cfg.Global = parseGlobalCfg(yCfg)
	cfg.Producer = yCfg.Producer
	cfg.Promexporters = parseExpCfg(yCfg)

	return cfg
}

func loadCfgFromFile(fName string) appConfig {
	yCfg := appConfig{}

	// Setting global and producer defaults
	yCfg.Global = Config{
		MetricsAddr: ":9310",
		MetricsPath: "/metrics",
		CreateFifo:  false,
	}

	yCfg.Producer = producer.Config{
		Input:      "stdin",
		DbAsn:      "",
		DbCountry:  "",
		Normalize:  true,
		SrOverride: -1,
	}

	// Read config from file
	f, err := ioutil.ReadFile(fName)
	if err != nil {
		log.Warning("cannot open config file. using default values...")
	}

	err = yaml.Unmarshal(f, &yCfg)
	if err != nil {
		log.Warning(err)
		log.Warning("using default values...")
	}

	return yCfg
}

func parseGlobalCfg(yCfg appConfig) Config {
	// Create fifo if required
	if yCfg.Global.CreateFifo && yCfg.Producer.Input != "stdin" {
		_ = syscall.Mkfifo(yCfg.Producer.Input, 0o666)
		_ = os.Chmod(yCfg.Producer.Input, 0o666)
	}

	return yCfg.Global
}

func parseExpCfg(yCfg appConfig) []promexp.Config {
	cfg := make([]promexp.Config, 0, len(yCfg.Promexporters))

	for i, v := range yCfg.Promexporters {
		// Setting PromExporters defaults
		cfg = append(cfg, promexp.Config{
			MetricsName:  fmt.Sprintf("pexp%d", i),
			MinBps:       v.MinBps,
			MinPps:       v.MinPps,
			FlowLife:     "5m",
			MaxScrapeInt: "2m",
			LabelSet:     []string{"SamplerAddress"},
		})

		if v.MetricsName != "" {
			cfg[i].MetricsName = fmt.Sprintf("gobi_%s", v.MetricsName)
		}

		if v.FlowLife != "" {
			_, err := time.ParseDuration(v.FlowLife)
			if err == nil {
				cfg[i].FlowLife = v.FlowLife
			}
		}

		if v.MaxScrapeInt != "" {
			_, err := time.ParseDuration(v.MaxScrapeInt)
			if err == nil {
				cfg[i].MaxScrapeInt = v.MaxScrapeInt
			}
		}

		if v.LabelSet != nil {
			cfg[i].LabelSet = formatLabelSet(v.LabelSet)
		}
	}

	return cfg
}

func formatLabelSet(labelSet []string) []string {
	labelKeys := make(map[string]bool, len(labelSet))
	out := make([]string, 0, len(labelSet))

	// Remove duplicates and LowerCase everything
	for _, item := range labelSet {
		if _, ok := labelKeys[item]; !ok {
			labelKeys[item] = true
			item = strings.ToLower(item)
			out = append(out, item)
		}
	}

	return out
}
