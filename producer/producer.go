// Package producer listen the protoBuf data stream coming from GoFlow2 and convert it in a Flow stream.
//
//	It can listen from stdin or a named pipe. It outputs a stream of Flow objects via interface.
//	It enriches flow data with Country, ASN, L3 proto and L4 proto and service extended infos.
package producer

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/automixer/gobi/db"
	"github.com/golang/protobuf/proto"
	"github.com/netsampler/goflow2/pb"
	log "github.com/sirupsen/logrus"
)

const maxConsumers = 4

// Flow holds a single flow dataset with arrival time.
type Flow struct {
	Fields   map[string]string
	Bytes    uint64
	Packets  uint64
	TimeRcvd time.Time
}

type Config struct {
	Input       string
	DbAsn       string
	DbCountry   string
	Normalize   bool
	SrOverride  int
	NoPortName  bool
	NoProtoName bool
	NoEtypeName bool
}

type GobiGf2 struct {
	db.GobiDb
	normalize  bool
	srOverride int
	consumers  []Consumer
	inRdr      *bufio.Reader
	inPipe     *os.File
	flowCh     chan Flow
	consCh     chan Consumer
	doneCh     chan struct{}
}

type Consumer interface {
	Consume(flow Flow)
}

func New(config Config) (*GobiGf2, func()) {
	gobi := &GobiGf2{}

	gobi.NoPortName = config.NoPortName
	gobi.NoProtoName = config.NoProtoName
	gobi.NoEtypeName = config.NoEtypeName
	gobi.normalize = config.Normalize
	gobi.srOverride = config.SrOverride
	gobi.consumers = make([]Consumer, 0, maxConsumers)
	gobi.OpenDbs(config.DbAsn, config.DbCountry)

	if config.Input == "stdin" {
		gobi.inRdr = bufio.NewReader(os.Stdin)
	} else {
		inPipe, err := os.OpenFile(config.Input, os.O_RDWR, os.ModeNamedPipe)
		if err != nil {
			log.Fatal("cannot open input pipe")
		}
		gobi.inPipe = inPipe
		gobi.inRdr = bufio.NewReader(gobi.inPipe)
	}

	gobi.flowCh = make(chan Flow)
	gobi.consCh = make(chan Consumer)
	gobi.doneCh = make(chan struct{})

	go gobi.ctrlRoutine()
	go gobi.msgRoutine()

	return gobi, func() {
		if gobi.inPipe != nil {
			_ = gobi.inPipe.Close()
		}
		close(gobi.doneCh)
		close(gobi.consCh)
		close(gobi.flowCh)
		gobi.CloseDbs()
	}
}

func (g *GobiGf2) Register(consumer Consumer) error {
	if consumer == nil {
		return errors.New("consumer must be a valid object")
	}

	if len(g.consumers) >= maxConsumers {
		return errors.New(fmt.Sprintf("max %d consumers are supported", maxConsumers))
	}
	g.consCh <- consumer
	return nil
}

func (g *GobiGf2) ctrlRoutine() {
	var flow Flow
	var cons Consumer
	for {
		select {
		case <-g.doneCh:
			return

		case flow = <-g.flowCh:
			for _, v := range g.consumers {
				if v != nil {
					v.Consume(flow)
				}
			}

		case cons = <-g.consCh:
			g.consumers = append(g.consumers, cons)
		}
	}
}

func (g *GobiGf2) msgRoutine() {
	msg := &flowpb.FlowMessage{}
	for {
		msg.Reset()
		bMsgLen, err := g.inRdr.Peek(binary.MaxVarintLen64)
		if err != nil && err != io.EOF {
			log.Error(err)
			log.Error("i/o error. quitting producer...")
			return
		}
		if err == io.EOF {
			continue
		}

		msgLen, vn := proto.DecodeVarint(bMsgLen)
		if msgLen == 0 {
			log.Warning("cannot decode protobuf varint")
			continue
		}

		_, err = g.inRdr.Discard(vn)
		if err != nil {
			log.Warning(err)
			continue
		}

		binMsg := make([]byte, msgLen)

		_, err = io.ReadFull(g.inRdr, binMsg)
		if err != nil && err != io.EOF {
			log.Warning(err)
			continue
		}
		if err == io.EOF {
			continue
		}

		binMsg = bytes.TrimSuffix(binMsg, []byte("\n"))

		err = proto.Unmarshal(binMsg, msg)
		if err != nil {
			log.Warning(err)
			continue
		}

		g.flowCh <- g.newFlow(msg)
	}
}

func (g *GobiGf2) newFlow(msg *flowpb.FlowMessage) Flow {
	f := Flow{
		Fields: make(map[string]string, 19),
	}

	f.TimeRcvd = time.Now()
	f.Fields["type"] = msg.Type.String()
	f.Fields["flowdirection"] = g.FindDirection(msg.FlowDirection)
	f.Fields["sampleraddress"] = g.FindIpAddr(msg.SamplerAddress)
	f.Fields["srcaddr"] = g.FindIpAddr(msg.SrcAddr)
	f.Fields["dstaddr"] = g.FindIpAddr(msg.DstAddr)
	f.Fields["etype"] = g.FindEtype(msg.Etype)
	f.Fields["proto"] = g.FindProto(msg.Proto)
	f.Fields["srcport"] = g.FindSvc(msg.Proto, msg.SrcPort)
	f.Fields["dstport"] = g.FindSvc(msg.Proto, msg.DstPort)
	f.Fields["inif"] = fmt.Sprint(msg.InIf)
	f.Fields["outif"] = fmt.Sprint(msg.OutIf)
	f.Fields["srcas"] = g.FindASN(msg.SrcAddr, msg.SrcAS)
	f.Fields["dstas"] = g.FindASN(msg.DstAddr, msg.DstAS)
	f.Fields["nexthop"] = g.FindIpAddr(msg.NextHop)
	f.Fields["nexthopas"] = g.FindASN(msg.NextHop, msg.NextHopAS)
	f.Fields["srcnet"] = g.FindNetwork(msg.SrcAddr, msg.SrcNet)
	f.Fields["dstnet"] = g.FindNetwork(msg.DstAddr, msg.DstNet)
	f.Fields["srccountry"] = g.FindCountry(msg.SrcAddr)
	f.Fields["dstcountry"] = g.FindCountry(msg.DstAddr)

	if g.srOverride >= 0 && g.normalize {
		msg.SamplingRate = uint64(g.srOverride)
	}

	if msg.SamplingRate > 1 && g.normalize {
		f.Bytes = msg.Bytes * msg.SamplingRate
		f.Packets = msg.Packets * msg.SamplingRate
	} else {
		f.Bytes = msg.Bytes
		f.Packets = msg.Packets
	}
	return f
}
