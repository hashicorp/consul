package types

import (
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/internal/resource/resourcetest"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/consul/proto/private/prototest"
)

func TestSimplifyFailoverPolicy(t *testing.T) {
	registry := resource.NewRegistry()
	Register(registry)

	// svc      *pbcatalog.Service
	// failover *pbcatalog.FailoverPolicy
	type testcase struct {
		svc      *pbresource.Resource
		failover *pbresource.Resource
		expect   *pbresource.Resource
	}
	run := func(t *testing.T, tc testcase) {
		// Ensure we only use valid inputs.
		resourcetest.ValidateAndNormalize(t, registry, tc.svc)
		resourcetest.ValidateAndNormalize(t, registry, tc.failover)
		resourcetest.ValidateAndNormalize(t, registry, tc.expect)

		svc := resourcetest.MustDecode[pbcatalog.Service, *pbcatalog.Service](t, tc.svc)
		failover := resourcetest.MustDecode[pbcatalog.FailoverPolicy, *pbcatalog.FailoverPolicy](t, tc.failover)
		expect := resourcetest.MustDecode[pbcatalog.FailoverPolicy, *pbcatalog.FailoverPolicy](t, tc.expect)

		inputFailoverCopy := proto.Clone(failover.Data).(*pbcatalog.FailoverPolicy)

		got := SimplifyFailoverPolicy(svc.Data, failover.Data)
		prototest.AssertDeepEqual(t, expect.Data, got)

		// verify input was not altered
		prototest.AssertDeepEqual(t, inputFailoverCopy, failover.Data)
	}

	newPort := func(name string, virtualPort uint32, protocol pbcatalog.Protocol) *pbcatalog.ServicePort {
		return &pbcatalog.ServicePort{
			VirtualPort: virtualPort,
			TargetPort:  name,
			Protocol:    protocol,
		}
	}

	newRef := func(typ *pbresource.Type, name string) *pbresource.Reference {
		return resourcetest.Resource(typ, name).Reference("")
	}

	cases := map[string]testcase{
		"explicit with port aligned defaulting": {
			svc: resourcetest.Resource(ServiceType, "api").
				WithData(t, &pbcatalog.Service{
					Ports: []*pbcatalog.ServicePort{
						newPort("http", 8080, pbcatalog.Protocol_PROTOCOL_HTTP),
						newPort("rest", 8282, pbcatalog.Protocol_PROTOCOL_HTTP2),
					},
				}).
				Build(),
			failover: resourcetest.Resource(FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"http": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(ServiceType, "api-backup"),
									Port: "www",
								},
								{
									Ref: newRef(ServiceType, "api-double-backup"),
								},
							},
						},
					},
				}).
				Build(),
			expect: resourcetest.Resource(FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"http": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(ServiceType, "api-backup"),
									Port: "www",
								},
								{
									Ref:  newRef(ServiceType, "api-double-backup"),
									Port: "http", // port defaulted
								},
							},
						},
					},
				}).
				Build(),
		},
		"implicit port explosion": {
			svc: resourcetest.Resource(ServiceType, "api").
				WithData(t, &pbcatalog.Service{
					Ports: []*pbcatalog.ServicePort{
						newPort("http", 8080, pbcatalog.Protocol_PROTOCOL_HTTP),
						newPort("rest", 8282, pbcatalog.Protocol_PROTOCOL_HTTP2),
					},
				}).
				Build(),
			failover: resourcetest.Resource(FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					Config: &pbcatalog.FailoverConfig{
						Destinations: []*pbcatalog.FailoverDestination{
							{
								Ref: newRef(ServiceType, "api-backup"),
							},
							{
								Ref: newRef(ServiceType, "api-double-backup"),
							},
						},
					},
				}).
				Build(),
			expect: resourcetest.Resource(FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"http": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(ServiceType, "api-backup"),
									Port: "http",
								},
								{
									Ref:  newRef(ServiceType, "api-double-backup"),
									Port: "http",
								},
							},
						},
						"rest": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(ServiceType, "api-backup"),
									Port: "rest",
								},
								{
									Ref:  newRef(ServiceType, "api-double-backup"),
									Port: "rest",
								},
							},
						},
					},
				}).
				Build(),
		},
		"mixed port explosion with skip": {
			svc: resourcetest.Resource(ServiceType, "api").
				WithData(t, &pbcatalog.Service{
					Ports: []*pbcatalog.ServicePort{
						newPort("http", 8080, pbcatalog.Protocol_PROTOCOL_HTTP),
						newPort("rest", 8282, pbcatalog.Protocol_PROTOCOL_HTTP2),
					},
				}).
				Build(),
			failover: resourcetest.Resource(FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					Config: &pbcatalog.FailoverConfig{
						Destinations: []*pbcatalog.FailoverDestination{
							{
								Ref: newRef(ServiceType, "api-backup"),
							},
							{
								Ref: newRef(ServiceType, "api-double-backup"),
							},
						},
					},
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"rest": {
							Mode:          pbcatalog.FailoverMode_FAILOVER_MODE_ORDER_BY_LOCALITY,
							Regions:       []string{"us", "eu"},
							SamenessGroup: "sameweb",
						},
					},
				}).
				Build(),
			expect: resourcetest.Resource(FailoverPolicyType, "api").
				WithData(t, &pbcatalog.FailoverPolicy{
					PortConfigs: map[string]*pbcatalog.FailoverConfig{
						"http": {
							Destinations: []*pbcatalog.FailoverDestination{
								{
									Ref:  newRef(ServiceType, "api-backup"),
									Port: "http",
								},
								{
									Ref:  newRef(ServiceType, "api-double-backup"),
									Port: "http",
								},
							},
						},
						"rest": {
							Mode:          pbcatalog.FailoverMode_FAILOVER_MODE_ORDER_BY_LOCALITY,
							Regions:       []string{"us", "eu"},
							SamenessGroup: "sameweb",
						},
					},
				}).
				Build(),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
