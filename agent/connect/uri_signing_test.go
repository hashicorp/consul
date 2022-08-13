package connect

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpiffeIDSigningForCluster(t *testing.T) {
	// For now it should just append .consul to the ID.
	id := SpiffeIDSigningForCluster(TestClusterID)
	assert.Equal(t, id.URI().String(), "spiffe://"+TestClusterID+".consul")
}

// fakeCertURI is a CertURI implementation that our implementation doesn't know
// about
type fakeCertURI string

func (f fakeCertURI) URI() *url.URL {
	u, _ := url.Parse(string(f))
	return u
}
func TestSpiffeIDSigning_CanSign(t *testing.T) {

	testSigning := &SpiffeIDSigning{
		ClusterID: TestClusterID,
		Domain:    "consul",
	}

	tests := []struct {
		name  string
		id    *SpiffeIDSigning
		input CertURI
		want  bool
	}{
		{
			name:  "same signing ID",
			id:    testSigning,
			input: testSigning,
			want:  true,
		},
		{
			name: "other signing ID",
			id:   testSigning,
			input: &SpiffeIDSigning{
				ClusterID: "fakedomain",
				Domain:    "consul",
			},
			want: false,
		},
		{
			name: "different TLD signing ID",
			id:   testSigning,
			input: &SpiffeIDSigning{
				ClusterID: TestClusterID,
				Domain:    "evil",
			},
			want: false,
		},
		{
			name:  "nil",
			id:    testSigning,
			input: nil,
			want:  false,
		},
		{
			name:  "unrecognised  CertURI implementation",
			id:    testSigning,
			input: fakeCertURI("spiffe://foo.bar/baz"),
			want:  false,
		},
		{
			name:  "service - good",
			id:    testSigning,
			input: &SpiffeIDService{Host: TestClusterID + ".consul", Namespace: "default", Datacenter: "dc1", Service: "web"},
			want:  true,
		},
		{
			name:  "service - good midex case",
			id:    testSigning,
			input: &SpiffeIDService{Host: strings.ToUpper(TestClusterID) + ".CONsuL", Namespace: "defAUlt", Datacenter: "dc1", Service: "WEB"},
			want:  true,
		},
		{
			name:  "service - different cluster",
			id:    testSigning,
			input: &SpiffeIDService{Host: "55555555-4444-3333-2222-111111111111.consul", Namespace: "default", Datacenter: "dc1", Service: "web"},
			want:  false,
		},
		{
			name:  "service - different TLD",
			id:    testSigning,
			input: &SpiffeIDService{Host: TestClusterID + ".fake", Namespace: "default", Datacenter: "dc1", Service: "web"},
			want:  false,
		},
		{
			name:  "mesh gateway - good",
			id:    testSigning,
			input: &SpiffeIDMeshGateway{Host: TestClusterID + ".consul", Datacenter: "dc1"},
			want:  true,
		},
		{
			name:  "mesh gateway - good midex case",
			id:    testSigning,
			input: &SpiffeIDMeshGateway{Host: strings.ToUpper(TestClusterID) + ".CONsuL", Datacenter: "dc1"},
			want:  true,
		},
		{
			name:  "mesh gateway - different cluster",
			id:    testSigning,
			input: &SpiffeIDMeshGateway{Host: "55555555-4444-3333-2222-111111111111.consul", Datacenter: "dc1"},
			want:  false,
		},
		{
			name:  "mesh gateway - different TLD",
			id:    testSigning,
			input: &SpiffeIDMeshGateway{Host: TestClusterID + ".fake", Datacenter: "dc1"},
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.id.CanSign(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
