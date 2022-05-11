// Package db implements a set of flows enriching functions
package db

import (
	"fmt"
	"net"

	"github.com/oschwald/geoip2-golang"
	log "github.com/sirupsen/logrus"
	"github.com/thediveo/netdb"
)

const msgNotAv = "not-av"
const msgNotStd = "not-std"

type GobiDb struct {
	maxMindASN     *geoip2.Reader
	maxMindCountry *geoip2.Reader
}

func (g *GobiDb) OpenDbs(asn, country string) {
	var dbType string

	if len(asn) >= 4 {
		dbType = asn[len(asn)-4:]
		switch dbType {
		case "mmdb":
			mmRdr, err := geoip2.Open(asn)
			if err != nil {
				log.Warning(err)
			} else {
				g.maxMindASN = mmRdr
			}
		default:
			log.Warning("unsupported asn db format")
		}
	}

	if len(country) >= 4 {
		dbType = country[len(country)-4:]
		switch dbType {
		case "mmdb":
			mmRdr, err := geoip2.Open(country)
			if err != nil {
				log.Warning(err)
			} else {
				g.maxMindCountry = mmRdr
			}
		default:
			log.Warning("unsupported country db format")
		}
	}
}

func (g *GobiDb) CloseDb() {
	if g.maxMindASN != nil {
		_ = g.maxMindASN.Close()
	}

	if g.maxMindCountry != nil {
		_ = g.maxMindCountry.Close()
	}
}

func (g *GobiDb) FindASN(ipAddr []byte, asn uint32) string {
	out := fmt.Sprintf("AS%d", asn)
	if g.maxMindASN != nil {
		entry, err := g.maxMindASN.ASN(ipAddr)
		if err == nil && entry.AutonomousSystemNumber != 0 {
			out = fmt.Sprintf("%s AS%d", entry.AutonomousSystemOrganization, entry.AutonomousSystemNumber)
		}
	}
	return out
}

func (g *GobiDb) FindCountry(ipAddr []byte) string {
	var out string
	if g.maxMindCountry != nil {
		entry, err := g.maxMindCountry.Country(ipAddr)
		if err == nil {
			out = entry.Country.IsoCode
		}
	}
	if out == "" {
		out = "ZZ"
	}
	return out
}

func (g *GobiDb) FindDirection(dir uint32) string {
	var out string
	switch dir {
	case 0:
		out = "in"
	case 1:
		out = "out"
	default:
		out = msgNotAv
	}
	return out
}

func (g *GobiDb) FindProto(pNumber uint32) string {
	out := msgNotStd
	pPtr := netdb.ProtocolByNumber(uint8(pNumber))
	if pPtr != nil {
		out = pPtr.Name
	}
	return out
}

func (g *GobiDb) FindSvc(pNumber, port uint32) string {
	out := msgNotStd
	switch pNumber {
	case 6: // tcp
		sPtr := netdb.ServiceByPort(int(port), "tcp")
		if sPtr != nil {
			out = sPtr.Name
		}
	case 17: // udp
		sPtr := netdb.ServiceByPort(int(port), "udp")
		if sPtr != nil {
			out = sPtr.Name
		}
	}
	return out
}

func (g *GobiDb) FindNetwork(ipAddr []byte, mask uint32) string {
	cidr := net.IP.String(ipAddr) + "/" + fmt.Sprint(mask)
	_, subnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "invalid"
	}
	return subnet.String()
}

func (g *GobiDb) FindEtype(eType uint32) string {
	out := msgNotStd
	switch eType {
	case 0x0800:
		out = "IPv4"
	case 0x0806:
		out = "ARP"
	case 0x8100:
		out = "802.1q"
	case 0x86dd:
		out = "IPv6"
	case 0x8809:
		out = "Slow Protocols"
	case 0x8847:
		out = "MPLS Unicast"
	case 0x8848:
		out = "MPLS Multicast"
	case 0x8863:
		out = "PPPoE Discovery"
	case 0x8864:
		out = "PPPoE Session"
	case 0x88a8:
		out = "QinQ"
	case 0x88cc:
		out = "LLDP"
	case 0x88e5:
		out = "MACsec"
	case 0x88e7:
		out = "PBB"
	case 0x88f7:
		out = "PTP"
	case 0x8906:
		out = "FCoE"
	}
	return out
}

func (g *GobiDb) FindIpAddr(ipAddr []byte) string {
	out := net.IP.String(ipAddr)
	if out == "<nil>" {
		out = msgNotAv
	}
	return out
}
