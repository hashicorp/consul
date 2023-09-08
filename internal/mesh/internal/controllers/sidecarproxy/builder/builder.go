package builder

import (
	pbmesh "github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbmesh/v1alpha1/pbproxystate"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

// Builder builds a ProxyStateTemplate.
type Builder struct {
	id                 *pbresource.ID
	proxyStateTemplate *pbmesh.ProxyStateTemplate
	proxyCfg           *pbmesh.ProxyConfiguration
	trustDomain        string
	localDatacenter    string

	outboundListenerBuilder *ListenerBuilder
}

func New(id *pbresource.ID,
	identity *pbresource.Reference,
	trustDomain string,
	dc string,
	proxyCfg *pbmesh.ProxyConfiguration,
) *Builder {

	return &Builder{
		id:              id,
		trustDomain:     trustDomain,
		localDatacenter: dc,
		proxyCfg:        proxyCfg,
		proxyStateTemplate: &pbmesh.ProxyStateTemplate{
			ProxyState: &pbmesh.ProxyState{
				Identity:  identity,
				Clusters:  make(map[string]*pbproxystate.Cluster),
				Endpoints: make(map[string]*pbproxystate.Endpoints),
			},
			RequiredEndpoints:        make(map[string]*pbproxystate.EndpointRef),
			RequiredLeafCertificates: make(map[string]*pbproxystate.LeafCertificateRef),
			RequiredTrustBundles:     make(map[string]*pbproxystate.TrustBundleRef),
		},
	}
}

func (b *Builder) Build() *pbmesh.ProxyStateTemplate {
	return b.proxyStateTemplate
}

type ListenerBuilder struct {
	listener *pbproxystate.Listener
	builder  *Builder
}

func (b *Builder) NewListenerBuilder(l *pbproxystate.Listener) *ListenerBuilder {
	return &ListenerBuilder{
		listener: l,
		builder:  b,
	}
}

func (l *ListenerBuilder) buildListener() *Builder {
	l.builder.proxyStateTemplate.ProxyState.Listeners = append(l.builder.proxyStateTemplate.ProxyState.Listeners, l.listener)

	return l.builder
}
