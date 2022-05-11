// Package promexp receives enriched flow data from Producer, stores it in a temporary buffer and
// waits for a Prometheus scrape event for further processing.
// At that time promexp executes these tasks:
//  - Checks out the accumulated flows from the temporary buffer.
//  - Aggregates those flows over a user defined label set.
//  - Checks if aggregates are compliant with a minimum user defined data rate. If not, they are
//    added to the untracked counters and removed from table.
//  - Checks the remaining flows in table for age. The expired ones are evicted.
//  - Exports the remaining flows to the Prometheus client.
package promexp

import (
	"sync"
	"time"

	"github.com/automixer/gobi/producer"
	"github.com/prometheus/client_golang/prometheus"
)

const fTableInitSize = 4096

type spinBuff struct {
	buffer   [2][]producer.Flow
	selector byte
	watchdog *time.Timer
	timeout  time.Duration
	lock     sync.Mutex
}

func newSpinBuff(maxLife time.Duration) *spinBuff {
	sb := &spinBuff{timeout: maxLife}
	sb.watchdog = time.AfterFunc(maxLife, sb.flush)
	return sb
}

func (s *spinBuff) flush() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.buffer[s.selector] = nil
	s.selector = 1 - s.selector // rotate selector
	s.watchdog.Reset(s.timeout)
}

func (s *spinBuff) addFlow(flow producer.Flow) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.buffer[s.selector] = append(s.buffer[s.selector], flow)
}

func (s *spinBuff) checkOut() []producer.Flow {
	s.lock.Lock()
	buf := s.buffer[s.selector]
	s.buffer[s.selector] = nil
	s.selector = 1 - s.selector // rotate selector
	s.lock.Unlock()

	s.watchdog.Stop()
	s.watchdog.Reset(s.timeout)
	return buf
}

type Config struct {
	MetricsName  string
	MinBps       uint64
	MinPps       uint64
	FlowLife     string
	MaxScrapeInt string
	LabelSet     []string
}

type flowTable map[string]producer.Flow

type GobiProm struct {
	metricsName  string
	minBps       uint64
	minPps       uint64
	flowLife     time.Duration
	maxScrapeInt time.Duration
	labelSet     []string

	fTable       flowTable
	spinB        *spinBuff
	bytesDesc    *prometheus.Desc
	packetsDesc  *prometheus.Desc
	ubytesDesc   *prometheus.Desc
	upacketsDesc *prometheus.Desc
	ubytesCnt    uint64
	upacketsCnt  uint64
	lastScrape   time.Time
}

func New(cfg Config) *GobiProm {
	fc := &GobiProm{}

	fc.metricsName = cfg.MetricsName
	fc.minBps = cfg.MinBps
	fc.minPps = cfg.MinPps
	fc.flowLife, _ = time.ParseDuration(cfg.FlowLife)
	fc.maxScrapeInt, _ = time.ParseDuration(cfg.MaxScrapeInt)
	fc.labelSet = cfg.LabelSet

	fc.fTable = make(flowTable, fTableInitSize)
	fc.spinB = newSpinBuff(fc.maxScrapeInt)
	fc.bytesDesc = prometheus.NewDesc(fc.metricsName+"_bytes", "", fc.labelSet, nil)
	fc.packetsDesc = prometheus.NewDesc(fc.metricsName+"_packets", "", fc.labelSet, nil)
	fc.ubytesDesc = prometheus.NewDesc(fc.metricsName+"_untracked_bytes", "", nil, nil)
	fc.upacketsDesc = prometheus.NewDesc(fc.metricsName+"_untracked_packets", "", nil, nil)
	fc.lastScrape = time.Now()
	prometheus.MustRegister(fc)

	return fc
}

func (g *GobiProm) mergeToMainTable(newTable flowTable) {
	for fID, f := range newTable {
		_, ok := g.fTable[fID]
		if ok {
			// Already in table. Sum counters
			f.Bytes += g.fTable[fID].Bytes
			f.Packets += g.fTable[fID].Packets
		}
		g.fTable[fID] = f
	}
}

func (g *GobiProm) aggregateFlows(flows []producer.Flow) flowTable {
	out := make(flowTable, len(flows))
	for _, flow := range flows {
		var flowID string

		for _, label := range g.labelSet {
			flowID += flow.Fields[label]
		}

		_, ok := out[flowID]
		if ok {
			// Already in table. Sum counters
			flow.Bytes += out[flowID].Bytes
			flow.Packets += out[flowID].Packets
		}

		out[flowID] = flow
	}
	return out
}

func (g *GobiProm) pruneUnderRate(ft *flowTable, interval uint64) {
	for fID, f := range *ft {
		var rateBps, ratePps uint64

		if interval > 0 {
			rateBps = f.Bytes / interval * 8
			ratePps = f.Packets / interval
		}

		if rateBps < g.minBps || ratePps < g.minPps {
			g.ubytesCnt += f.Bytes
			g.upacketsCnt += f.Packets
			delete(*ft, fID)
		}
	}
}

func (g *GobiProm) Consume(flow producer.Flow) {
	g.spinB.addFlow(flow)
}

func (g *GobiProm) Describe(ch chan<- *prometheus.Desc) {
	ch <- g.bytesDesc
	ch <- g.packetsDesc
	ch <- g.ubytesDesc
	ch <- g.upacketsDesc
}

// Collect implements the Prometheus Collect interface.
// This method is called by Prometheus Client each time the exporter is scraped.
func (g *GobiProm) Collect(ch chan<- prometheus.Metric) {
	inFlows := g.spinB.checkOut()
	scrapeInterval := uint64(time.Since(g.lastScrape)) / 1_000_000_000
	g.lastScrape = time.Now()

	// Aggregate input flows
	inTable := g.aggregateFlows(inFlows)

	// Minimum rate check
	if g.minBps > 0 || g.minPps > 0 {
		g.pruneUnderRate(&inTable, scrapeInterval)
	}

	// Export untracked flows
	ch <- prometheus.MustNewConstMetric(g.ubytesDesc, prometheus.CounterValue, float64(g.ubytesCnt))
	ch <- prometheus.MustNewConstMetric(g.upacketsDesc, prometheus.CounterValue, float64(g.upacketsCnt))

	// Merge new flows to fTable
	g.mergeToMainTable(inTable)

	// Export tracked flows
	for fID, fVal := range g.fTable {
		// Timeout flows
		if time.Since(fVal.TimeRcvd) >= g.flowLife {
			delete(g.fTable, fID)
			continue
		}

		// Extract label values
		lVals := make([]string, len(g.labelSet))
		for i, j := range g.labelSet {
			lVals[i] = fVal.Fields[j]
		}

		// Send to Prom client
		ch <- prometheus.MustNewConstMetric(g.bytesDesc, prometheus.CounterValue, float64(fVal.Bytes), lVals...)
		ch <- prometheus.MustNewConstMetric(g.packetsDesc, prometheus.CounterValue, float64(fVal.Packets), lVals...)
	}
}
