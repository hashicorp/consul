package connect

import (
	"net/url"
	"strings"
	"testing"

	"github.com/hashicorp/consul/agent/structs"

	"github.com/stretchr/testify/assert"
)

// Signing ID should never authorize
func TestSpiffeIDSigningAuthorize(t *testing.T) {
	var id SpiffeIDSigning
	auth, ok := id.Authorize(nil)
	assert.False(t, auth)
	assert.True(t, ok)
}

func TestSpiffeIDSigningForCluster(t *testing.T) {
	// For now it should just append .consul to the ID.
	config := &structs.CAConfiguration{
		ClusterID: TestClusterID,
	}
	id := SpiffeIDSigningForCluster(config)
	assert.Equal(t, id.URI().String(), "spiffe://"+TestClusterID+".consul")
}

// fakeCertURI is a CertURI implementation that our implementation doesn't know
// about
type fakeCertURI string

func (f fakeCertURI) Authorize(*structs.Intention) (auth bool, match bool) {
	return false, false
}

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
			input: &SpiffeIDService{TestClusterID + ".consul", "default", "dc1", "web"},
			want:  true,
		},
		{
			name:  "service - good midex case",
			id:    testSigning,
			input: &SpiffeIDService{strings.ToUpper(TestClusterID) + ".CONsuL", "defAUlt", "dc1", "WEB"},
			want:  true,
		},
		{
			name:  "service - different cluster",
			id:    testSigning,
			input: &SpiffeIDService{"55555555-4444-3333-2222-111111111111.consul", "default", "dc1", "web"},
			want:  false,
		},
		{
			name:  "service - different TLD",
			id:    testSigning,
			input: &SpiffeIDService{TestClusterID + ".fake", "default", "dc1", "web"},
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
