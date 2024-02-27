// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v2beta1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testrpc"
)

// Similar to TestDNS_ServiceLookup, but removes config for features unsupported in v2 and
// tests against DNS v2 and Catalog v2 explicitly using a resource API client.
func TestDNS_CatalogV2_Basic(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	var err error
	a := NewTestAgent(t, `experiments=["resource-apis"]`) // v2dns is implicit w/ resource-apis
	defer a.Shutdown()

	testrpc.WaitForRaftLeader(t, a.RPC, "dc1")

	client := a.delegate.ResourceServiceClient()

	// Smoke test for `consul-server` service.
	readResource(t, client, &pbresource.ID{
		Name:    structs.ConsulServiceName,
		Type:    pbcatalog.ServiceType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}, new(pbcatalog.Service))

	// Register a new service.
	dbServiceId := &pbresource.ID{
		Name:    "db",
		Type:    pbcatalog.ServiceType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}
	emptyServiceId := &pbresource.ID{
		Name:    "empty",
		Type:    pbcatalog.ServiceType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}
	dbService := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{"db-"},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort: "tcp",
				Protocol:   pbcatalog.Protocol_PROTOCOL_TCP,
			},
			{
				TargetPort: "admin",
				Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
			},
			{
				TargetPort: "mesh",
				Protocol:   pbcatalog.Protocol_PROTOCOL_MESH,
			},
		},
	}
	emptyService := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{"empty-"},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort: "tcp",
				Protocol:   pbcatalog.Protocol_PROTOCOL_TCP,
			},
			{
				TargetPort: "admin",
				Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
			},
			{
				TargetPort: "mesh",
				Protocol:   pbcatalog.Protocol_PROTOCOL_MESH,
			},
		},
	}
	dbServiceResource := &pbresource.Resource{
		Id:   dbServiceId,
		Data: toAny(t, dbService),
	}
	emptyServiceResource := &pbresource.Resource{
		Id:   emptyServiceId,
		Data: toAny(t, emptyService),
	}
	for _, r := range []*pbresource.Resource{dbServiceResource, emptyServiceResource} {
		_, err := client.Write(context.Background(), &pbresource.WriteRequest{Resource: r})
		if err != nil {
			t.Fatalf("failed to create the %s service: %v", r.Id.Name, err)
		}
	}

	// Validate services written.
	readResource(t, client, dbServiceId, new(pbcatalog.Service))
	readResource(t, client, emptyServiceId, new(pbcatalog.Service))

	// Register workloads.
	dbWorkloadId1 := &pbresource.ID{
		Name:    "db-1",
		Type:    pbcatalog.WorkloadType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}
	dbWorkloadId2 := &pbresource.ID{
		Name:    "db-2",
		Type:    pbcatalog.WorkloadType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}
	dbWorkloadId3 := &pbresource.ID{
		Name:    "db-3",
		Type:    pbcatalog.WorkloadType,
		Tenancy: resource.DefaultNamespacedTenancy(),
	}
	dbWorkloadPorts := map[string]*pbcatalog.WorkloadPort{
		"tcp": {
			Port:     12345,
			Protocol: pbcatalog.Protocol_PROTOCOL_TCP,
		},
		"admin": {
			Port:     23456,
			Protocol: pbcatalog.Protocol_PROTOCOL_HTTP,
		},
		"mesh": {
			Port:     20000,
			Protocol: pbcatalog.Protocol_PROTOCOL_MESH,
		},
	}
	dbWorkloadFn := func(ip string) *pbcatalog.Workload {
		return &pbcatalog.Workload{
			Addresses: []*pbcatalog.WorkloadAddress{
				{
					Host: ip,
				},
			},
			Identity: "test-identity",
			Ports:    dbWorkloadPorts,
		}
	}
	dbWorkload1 := dbWorkloadFn("172.16.1.1")
	_, err = client.Write(context.Background(), &pbresource.WriteRequest{Resource: &pbresource.Resource{
		Id:   dbWorkloadId1,
		Data: toAny(t, dbWorkload1),
	}})
	if err != nil {
		t.Fatalf("failed to create the %s workload: %v", dbWorkloadId1.Name, err)
	}
	dbWorkload2 := dbWorkloadFn("172.16.1.2")
	_, err = client.Write(context.Background(), &pbresource.WriteRequest{Resource: &pbresource.Resource{
		Id:   dbWorkloadId2,
		Data: toAny(t, dbWorkload2),
	}})
	if err != nil {
		t.Fatalf("failed to create the %s workload: %v", dbWorkloadId2.Name, err)
	}
	dbWorkload3 := dbWorkloadFn("2001:db8:85a3::8a2e:370:7334") // test IPv6
	_, err = client.Write(context.Background(), &pbresource.WriteRequest{Resource: &pbresource.Resource{
		Id:   dbWorkloadId3,
		Data: toAny(t, dbWorkload3),
	}})
	if err != nil {
		t.Fatalf("failed to create the %s workload: %v", dbWorkloadId2.Name, err)
	}

	// Validate workloads written.
	dbWorkloads := make(map[string]*pbcatalog.Workload)
	dbWorkloads["db-1"] = readResource(t, client, dbWorkloadId1, new(pbcatalog.Workload)).(*pbcatalog.Workload)
	dbWorkloads["db-2"] = readResource(t, client, dbWorkloadId2, new(pbcatalog.Workload)).(*pbcatalog.Workload)
	dbWorkloads["db-3"] = readResource(t, client, dbWorkloadId3, new(pbcatalog.Workload)).(*pbcatalog.Workload)

	// Ensure endpoints exist and have health status, which is required for inclusion in DNS results.
	retry.Run(t, func(r *retry.R) {
		endpoints := readResource(r, client, resource.ReplaceType(pbcatalog.ServiceEndpointsType, dbServiceId), new(pbcatalog.ServiceEndpoints)).(*pbcatalog.ServiceEndpoints)
		require.Equal(r, 3, len(endpoints.GetEndpoints()))
		for _, e := range endpoints.GetEndpoints() {
			require.True(r,
				// We only return results for passing and warning health checks.
				e.HealthStatus == pbcatalog.Health_HEALTH_PASSING || e.HealthStatus == pbcatalog.Health_HEALTH_WARNING,
				fmt.Sprintf("unexpected health status: %v", e.HealthStatus))
		}
	})

	// Test UDP and TCP clients.
	for _, client := range []*dns.Client{
		newDNSClient(false),
		newDNSClient(true),
	} {
		// Lookup a service without matching workloads, we should receive an SOA and no answers.
		questions := []string{
			"empty.service.consul.",
			"_empty._tcp.service.consul.",
		}
		for _, question := range questions {
			for _, dnsType := range []uint16{dns.TypeSRV, dns.TypeA, dns.TypeAAAA} {
				m := new(dns.Msg)
				m.SetQuestion(question, dnsType)

				in, _, err := client.Exchange(m, a.DNSAddr())
				if err != nil {
					t.Fatalf("err: %v", err)
				}

				require.Equal(t, 0, len(in.Answer), "Bad: %s", in.String())
				require.Equal(t, 0, len(in.Extra), "Bad: %s", in.String())
				require.Equal(t, 1, len(in.Ns), "Bad: %s", in.String())

				soaRec, ok := in.Ns[0].(*dns.SOA)
				require.True(t, ok, "Bad: %s", in.Ns[0].String())
				require.EqualValues(t, 0, soaRec.Hdr.Ttl, "Bad: %s", in.Ns[0].String())
			}
		}

		// Look up the service directly including all ports.
		questions = []string{
			"db.service.consul.",
			"_db._tcp.service.consul.", // RFC 2782 query. All ports are TCP, so this should return the same result.
		}
		for _, question := range questions {
			m := new(dns.Msg)
			m.SetQuestion(question, dns.TypeSRV)

			in, _, err := client.Exchange(m, a.DNSAddr())
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			// This check only runs for a TCP client because a UDP client will truncate the response.
			if client.Net == "tcp" {
				for portName, port := range dbWorkloadPorts {
					for workloadName, workload := range dbWorkloads {
						workloadTarget := fmt.Sprintf("%s.port.%s.workload.default.ns.default.ap.consul.", portName, workloadName)
						workloadHost := workload.Addresses[0].Host

						srvRec := findSrvAnswerForTarget(t, in, workloadTarget)
						require.EqualValues(t, port.Port, srvRec.Port, "Bad: %s", srvRec.String())
						require.EqualValues(t, 0, srvRec.Hdr.Ttl, "Bad: %s", srvRec.String())

						a := findAorAAAAForName(t, in, in.Extra, workloadTarget)
						require.Equal(t, workloadHost, a.AorAAAA.String(), "Bad: %s", a.Original.String())
						require.EqualValues(t, 0, a.Hdr.Ttl, "Bad: %s", a.Original.String())
					}
				}

				// Expect 1 result per port, per workload.
				require.Equal(t, 9, len(in.Answer), "answer count did not match expected\n\n%s", in.String())
				require.Equal(t, 9, len(in.Extra), "extra answer count did not match expected\n\n%s", in.String())
			} else {
				// Expect 1 result per port, per workload, up to the default limit of 3. In practice the results are truncated
				// at 2 because of the record byte size.
				require.Equal(t, 2, len(in.Answer), "answer count did not match expected\n\n%s", in.String())
				require.Equal(t, 2, len(in.Extra), "extra answer count did not match expected\n\n%s", in.String())
			}
		}

		// Look up the service directly by each port.
		for portName, port := range dbWorkloadPorts {
			question := fmt.Sprintf("%s.port.db.service.consul.", portName)

			for workloadName, workload := range dbWorkloads {
				workloadTarget := fmt.Sprintf("%s.port.%s.workload.default.ns.default.ap.consul.", portName, workloadName)
				workloadHost := workload.Addresses[0].Host

				m := new(dns.Msg)
				m.SetQuestion(question, dns.TypeSRV)

				in, _, err := client.Exchange(m, a.DNSAddr())
				if err != nil {
					t.Fatalf("err: %v", err)
				}

				srvRec := findSrvAnswerForTarget(t, in, workloadTarget)
				require.EqualValues(t, port.Port, srvRec.Port, "Bad: %s", srvRec.String())
				require.EqualValues(t, 0, srvRec.Hdr.Ttl, "Bad: %s", srvRec.String())

				a := findAorAAAAForName(t, in, in.Extra, workloadTarget)
				require.Equal(t, workloadHost, a.AorAAAA.String(), "Bad: %s", a.Original.String())
				require.EqualValues(t, 0, a.Hdr.Ttl, "Bad: %s", a.Original.String())

				// Expect 1 result per port.
				require.Equal(t, 3, len(in.Answer), "answer count did not match expected\n\n%s", in.String())
				require.Equal(t, 3, len(in.Extra), "extra answer count did not match expected\n\n%s", in.String())
			}
		}

		// Look up A/AAAA by service.
		questions = []string{
			"db.service.consul.",
		}
		for _, question := range questions {
			for workloadName, dnsType := range map[string]uint16{
				"db-1": dns.TypeA,
				"db-2": dns.TypeA,
				"db-3": dns.TypeAAAA,
			} {
				workload := dbWorkloads[workloadName]

				m := new(dns.Msg)
				m.SetQuestion(question, dnsType)

				in, _, err := client.Exchange(m, a.DNSAddr())
				if err != nil {
					t.Fatalf("err: %v", err)
				}

				workloadHost := workload.Addresses[0].Host

				a := findAorAAAAForAddress(t, in, in.Answer, workloadHost)
				require.Equal(t, question, a.Hdr.Name, "Bad: %s", a.Original.String())
				require.EqualValues(t, 0, a.Hdr.Ttl, "Bad: %s", a.Original.String())

				// Expect 1 answer per workload. For A records, expect 2 answers because there's 2 IPv4 workloads.
				if dnsType == dns.TypeA {
					require.Equal(t, 2, len(in.Answer), "answer count did not match expected\n\n%s", in.String())
				} else {
					require.Equal(t, 1, len(in.Answer), "answer count did not match expected\n\n%s", in.String())
				}
				require.Equal(t, 0, len(in.Extra), "extra answer count did not match expected\n\n%s", in.String())
			}
		}

		// Lookup a non-existing service, we should receive an SOA.
		questions = []string{
			"nodb.service.consul.",
			"nope.query.consul.", // prepared query is not supported in v2
		}
		for _, question := range questions {
			m := new(dns.Msg)
			m.SetQuestion(question, dns.TypeSRV)

			in, _, err := client.Exchange(m, a.DNSAddr())
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			require.Equal(t, 1, len(in.Ns), "Bad: %s", in.String())

			soaRec, ok := in.Ns[0].(*dns.SOA)
			require.True(t, ok, "Bad: %s", in.Ns[0].String())
			require.EqualValues(t, 0, soaRec.Hdr.Ttl, "Bad: %s", in.Ns[0].String())
		}

		// Lookup workloads directly with a port.
		for workloadName, dnsType := range map[string]uint16{
			"db-1": dns.TypeA,
			"db-2": dns.TypeA,
			"db-3": dns.TypeAAAA,
		} {
			for _, question := range []string{
				fmt.Sprintf("%s.workload.default.ns.default.ap.consul.", workloadName),
				fmt.Sprintf("tcp.port.%s.workload.default.ns.default.ap.consul.", workloadName),
				fmt.Sprintf("admin.port.%s.workload.default.ns.default.ap.consul.", workloadName),
				fmt.Sprintf("mesh.port.%s.workload.default.ns.default.ap.consul.", workloadName),
			} {
				workload := dbWorkloads[workloadName]
				workloadHost := workload.Addresses[0].Host

				m := new(dns.Msg)
				m.SetQuestion(question, dnsType)

				in, _, err := client.Exchange(m, a.DNSAddr())
				if err != nil {
					t.Fatalf("err: %v", err)
				}

				require.Equal(t, 1, len(in.Answer), "Bad: %s", in.String())

				a := findAorAAAAForName(t, in, in.Answer, question)
				require.Equal(t, workloadHost, a.AorAAAA.String(), "Bad: %s", a.Original.String())
				require.EqualValues(t, 0, a.Hdr.Ttl, "Bad: %s", a.Original.String())
			}
		}

		// Lookup a non-existing workload, we should receive an NXDOMAIN response.
		for _, aType := range []uint16{dns.TypeA, dns.TypeAAAA} {
			question := "unknown.workload.consul."

			m := new(dns.Msg)
			m.SetQuestion(question, aType)

			in, _, err := client.Exchange(m, a.DNSAddr())
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			require.Equal(t, 0, len(in.Answer), "Bad: %s", in.String())
			require.Equal(t, dns.RcodeNameError, in.Rcode, "Bad: %s", in.String())
		}
	}
}

func findSrvAnswerForTarget(t *testing.T, in *dns.Msg, target string) *dns.SRV {
	t.Helper()

	for _, a := range in.Answer {
		srvRec, ok := a.(*dns.SRV)
		if ok && srvRec.Target == target {
			return srvRec
		}
	}
	t.Fatalf("could not find SRV record for target: %s\n\n%s", target, in.String())
	return nil
}

func findAorAAAAForName(t *testing.T, in *dns.Msg, rrs []dns.RR, name string) *dnsAOrAAAA {
	t.Helper()

	for _, rr := range rrs {
		a := newAOrAAAA(t, rr)
		if a.Hdr.Name == name {
			return a
		}
	}
	t.Fatalf("could not find A/AAAA record for name: %s\n\n%+v", name, in.String())
	return nil
}

func findAorAAAAForAddress(t *testing.T, in *dns.Msg, rrs []dns.RR, address string) *dnsAOrAAAA {
	t.Helper()

	for _, rr := range rrs {
		a := newAOrAAAA(t, rr)
		if a.AorAAAA.String() == address {
			return a
		}
	}
	t.Fatalf("could not find A/AAAA record for address: %s\n\n%+v", address, in.String())
	return nil
}

func readResource(t retry.TestingTB, client pbresource.ResourceServiceClient, id *pbresource.ID, m proto.Message) proto.Message {
	t.Helper()

	retry.Run(t, func(r *retry.R) {
		res, err := client.Read(context.Background(), &pbresource.ReadRequest{Id: id})
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		data := res.GetResource()
		require.NotEmpty(r, data)

		err = data.Data.UnmarshalTo(m)
		require.NoError(r, err)
	})

	return m
}

func toAny(t retry.TestingTB, m proto.Message) *anypb.Any {
	t.Helper()
	a, err := anypb.New(m)
	if err != nil {
		t.Fatalf("could not convert proto to `any` message: %v", err)
	}
	return a
}

// dnsAOrAAAA unifies A and AAAA records for simpler testing when the IP type doesn't matter.
type dnsAOrAAAA struct {
	Original dns.RR
	Hdr      dns.RR_Header
	AorAAAA  net.IP
	isAAAA   bool
}

func newAOrAAAA(t *testing.T, rr dns.RR) *dnsAOrAAAA {
	t.Helper()

	if aRec, ok := rr.(*dns.A); ok {
		return &dnsAOrAAAA{
			Original: rr,
			Hdr:      aRec.Hdr,
			AorAAAA:  aRec.A,
			isAAAA:   false,
		}
	}
	if aRec, ok := rr.(*dns.AAAA); ok {
		return &dnsAOrAAAA{
			Original: rr,
			Hdr:      aRec.Hdr,
			AorAAAA:  aRec.AAAA,
			isAAAA:   true,
		}
	}

	t.Fatalf("Bad A or AAAA record: %#v", rr)
	return nil
}

func newDNSClient(tcp bool) *dns.Client {
	c := new(dns.Client)

	// Use TCP to avoid truncation of larger responses and
	// sidestep the default UDP size limit of 3 answers
	// set by config.DefaultSource() in agent/config/default.go.
	if tcp {
		c.Net = "tcp"
	}

	return c
}
