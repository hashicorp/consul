package metadata

import (
	"context"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/variables"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// Metadata implements collecting metadata information from all plugins that
// implement the Provider interface.
type Metadata struct {
	Zones     []string
	Providers []Provider
	Next      plugin.Handler
}

// Name implements the Handler interface.
func (m *Metadata) Name() string { return "metadata" }

// ServeDNS implements the plugin.Handler interface.
func (m *Metadata) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {

	ctx = context.WithValue(ctx, metadataKey{}, M{})
	md, _ := FromContext(ctx)

	state := request.Request{W: w, Req: r}
	if plugin.Zones(m.Zones).Matches(state.Name()) != "" {
		// Go through all Providers and collect metadata.
		for _, provider := range m.Providers {
			for _, varName := range provider.MetadataVarNames() {
				if val, ok := provider.Metadata(ctx, state, varName); ok {
					md.SetValue(varName, val)
				}
			}
		}
	}

	rcode, err := plugin.NextOrFailure(m.Name(), m.Next, ctx, w, r)

	return rcode, err
}

// MetadataVarNames implements the plugin.Provider interface.
func (m *Metadata) MetadataVarNames() []string { return variables.All }

// Metadata implements the plugin.Provider interface.
func (m *Metadata) Metadata(ctx context.Context, state request.Request, varName string) (interface{}, bool) {
	if val, err := variables.GetValue(state, varName); err == nil {
		return val, true
	}
	return nil, false
}
