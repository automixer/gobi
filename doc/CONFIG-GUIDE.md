# Gobi Configuration Guide

The complete list of supported configuration statement with default values:

```
# Section Global
global:
  metricsaddr: :9310 # Http server listen address.
  metricspath: /metrics # Http server metrics path.
  createfifo: false # If true the producer's input named pipe is created by Gobi.

# Section Producer
producer:
  input: stdin # Producer input device [stdin | <path to fifo>]
  dbasn: <path to ASN db>
  dbcountry: <path to Country db>
  normalize: true # If true, flows counters are adjusted with the received sample ratio.
  sroverride: -1 # Overrides the received NF sample ratio. Negative value disable the feature.

# Section PromExporters
promexporters:
  - metricsname: pexpX # Prom metric name will be: "gobi_pexp1_bytes"
    minbps: 0 # Minimum bps rate to be accounted as regular flow
    minpps: 0 # Minimum pps rate to be accounted as regular flow
    flowlife: 5m # Max flow life. Hours, minutes, seconds suffix accepted 
    maxscrapeint: 2m # Max Prometheus scrape interval. Hours, minutes, seconds suffix accepted. 

    # List of labels over wich aggregate flows (default: SamplerAddress)
    labelset: ["Type", "FlowDirection", "SamplerAddress", "SrcAddr", 
               "DstAddr", "Etype", "Proto", "SrcPort", "DstPort", 
               "InIf", "OutIf", "SrcAS", "DstAS", "NextHop", "NextHopAS", 
               "SrcNet", "DstNet", "SrcCountry", "DstCountry"]

  # up to 4 promexporter instances supported
  - metricsname: pexpX # Prom metric name will be: "gobi_pexp2_bytes"
    ....
```

The Gobi command line syntax:

```
gobi [OPTIONS]
-f 
    path to config file
-v
    print version
-ll
    set the log level (defaults to "info")
-lf
    set the log format (defaults to "text")
```
