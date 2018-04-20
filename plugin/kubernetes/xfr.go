package kubernetes

import (
	"math"
	"net"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"

	"context"

	"github.com/miekg/dns"
	api "k8s.io/api/core/v1"
)

const transferLength = 2000

// Serial implements the Transferer interface.
func (k *Kubernetes) Serial(state request.Request) uint32 { return uint32(k.APIConn.Modified()) }

// MinTTL implements the Transferer interface.
func (k *Kubernetes) MinTTL(state request.Request) uint32 { return 30 }

// Transfer implements the Transferer interface.
func (k *Kubernetes) Transfer(ctx context.Context, state request.Request) (int, error) {

	// Get all services.
	rrs := make(chan dns.RR)
	go k.transfer(rrs, state.Zone)

	records := []dns.RR{}
	for r := range rrs {
		records = append(records, r)
	}

	if len(records) == 0 {
		return dns.RcodeServerFailure, nil
	}

	ch := make(chan *dns.Envelope)
	tr := new(dns.Transfer)

	soa, err := plugin.SOA(k, state.Zone, state, plugin.Options{})
	if err != nil {
		return dns.RcodeServerFailure, nil
	}

	records = append(soa, records...)
	records = append(records, soa...)
	go func(ch chan *dns.Envelope) {
		j, l := 0, 0
		log.Infof("Outgoing transfer of %d records of zone %s to %s started", len(records), state.Zone, state.IP())
		for i, r := range records {
			l += dns.Len(r)
			if l > transferLength {
				ch <- &dns.Envelope{RR: records[j:i]}
				l = 0
				j = i
			}
		}
		if j < len(records) {
			ch <- &dns.Envelope{RR: records[j:]}
		}
		close(ch)
	}(ch)

	tr.Out(state.W, state.Req, ch)
	// Defer closing to the client
	state.W.Hijack()
	return dns.RcodeSuccess, nil
}

func (k *Kubernetes) transfer(c chan dns.RR, zone string) {

	defer close(c)

	zonePath := msg.Path(zone, "coredns")
	serviceList := k.APIConn.ServiceList()
	for _, svc := range serviceList {
		svcBase := []string{zonePath, Svc, svc.Namespace, svc.Name}
		switch svc.Spec.Type {
		case api.ServiceTypeClusterIP, api.ServiceTypeNodePort, api.ServiceTypeLoadBalancer:
			clusterIP := net.ParseIP(svc.Spec.ClusterIP)
			if clusterIP != nil {
				for _, p := range svc.Spec.Ports {

					s := msg.Service{Host: svc.Spec.ClusterIP, Port: int(p.Port), TTL: k.ttl}
					s.Key = strings.Join(svcBase, "/")

					// Change host from IP to Name for SRV records
					host := emitAddressRecord(c, s)
					s.Host = host

					// Need to generate this to handle use cases for peer-finder
					// ref: https://github.com/coredns/coredns/pull/823
					c <- s.NewSRV(msg.Domain(s.Key), 100)

					// As per spec unnamed ports do not have a srv record
					// https://github.com/kubernetes/dns/blob/master/docs/specification.md#232---srv-records
					if p.Name == "" {
						continue
					}

					s.Key = strings.Join(append(svcBase, strings.ToLower("_"+string(p.Protocol)), strings.ToLower("_"+string(p.Name))), "/")

					c <- s.NewSRV(msg.Domain(s.Key), 100)
				}

				//  Skip endpoint discovery if clusterIP is defined
				continue
			}

			endpointsList := k.APIConn.EpIndex(svc.Name + "." + svc.Namespace)

			for _, ep := range endpointsList {
				if ep.ObjectMeta.Name != svc.Name || ep.ObjectMeta.Namespace != svc.Namespace {
					continue
				}

				for _, eps := range ep.Subsets {
					srvWeight := calcSRVWeight(len(eps.Addresses))
					for _, addr := range eps.Addresses {
						s := msg.Service{Host: addr.IP, TTL: k.ttl}
						s.Key = strings.Join(svcBase, "/")
						// We don't need to change the msg.Service host from IP to Name yet
						// so disregard the return value here
						emitAddressRecord(c, s)

						s.Key = strings.Join(append(svcBase, endpointHostname(addr, k.endpointNameMode)), "/")
						// Change host from IP to Name for SRV records
						host := emitAddressRecord(c, s)
						s.Host = host

						for _, p := range eps.Ports {
							// As per spec unnamed ports do not have a srv record
							// https://github.com/kubernetes/dns/blob/master/docs/specification.md#232---srv-records
							if p.Name == "" {
								continue
							}

							s.Port = int(p.Port)

							s.Key = strings.Join(append(svcBase, strings.ToLower("_"+string(p.Protocol)), strings.ToLower("_"+string(p.Name))), "/")
							c <- s.NewSRV(msg.Domain(s.Key), srvWeight)
						}
					}
				}
			}

		case api.ServiceTypeExternalName:

			s := msg.Service{Key: strings.Join(svcBase, "/"), Host: svc.Spec.ExternalName, TTL: k.ttl}
			if t, _ := s.HostType(); t == dns.TypeCNAME {
				c <- s.NewCNAME(msg.Domain(s.Key), s.Host)
			}
		}
	}
	return
}

// emitAddressRecord generates a new A or AAAA record based on the msg.Service and writes it to
// a channel.
// emitAddressRecord returns the host name from the generated record.
func emitAddressRecord(c chan dns.RR, message msg.Service) string {
	ip := net.ParseIP(message.Host)
	var host string
	dnsType, _ := message.HostType()
	switch dnsType {
	case dns.TypeA:
		arec := message.NewA(msg.Domain(message.Key), ip)
		host = arec.Hdr.Name
		c <- arec
	case dns.TypeAAAA:
		arec := message.NewAAAA(msg.Domain(message.Key), ip)
		host = arec.Hdr.Name
		c <- arec
	}

	return host
}

// calcSrvWeight borrows the logic implemented in plugin.SRV for dynamically
// calculating the srv weight and priority
func calcSRVWeight(numservices int) uint16 {
	var services []msg.Service

	for i := 0; i < numservices; i++ {
		services = append(services, msg.Service{})
	}

	w := make(map[int]int)
	for _, serv := range services {
		weight := 100
		if serv.Weight != 0 {
			weight = serv.Weight
		}
		if _, ok := w[serv.Priority]; !ok {
			w[serv.Priority] = weight
			continue
		}
		w[serv.Priority] += weight
	}

	return uint16(math.Floor((100.0 / float64(w[0])) * 100))
}
