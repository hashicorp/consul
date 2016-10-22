package proxy

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mholt/caddy"
)

func TestHealthCheck(t *testing.T) {
	upstream := &staticUpstream{
		from:        "",
		Hosts:       testPool(),
		Policy:      &Random{},
		Spray:       nil,
		FailTimeout: 10 * time.Second,
		MaxFails:    1,
	}
	upstream.healthCheck()
	if upstream.Hosts[0].Down() {
		t.Error("Expected first host in testpool to not fail healthcheck.")
	}
	if !upstream.Hosts[1].Down() {
		t.Error("Expected second host in testpool to fail healthcheck.")
	}
}

func TestSelect(t *testing.T) {
	upstream := &staticUpstream{
		from:        "",
		Hosts:       testPool()[:3],
		Policy:      &Random{},
		FailTimeout: 10 * time.Second,
		MaxFails:    1,
	}
	upstream.Hosts[0].Unhealthy = true
	upstream.Hosts[1].Unhealthy = true
	upstream.Hosts[2].Unhealthy = true
	if h := upstream.Select(); h != nil {
		t.Error("Expected select to return nil as all host are down")
	}
	upstream.Hosts[2].Unhealthy = false
	if h := upstream.Select(); h == nil {
		t.Error("Expected select to not return nil")
	}
}

func TestRegisterPolicy(t *testing.T) {
	name := "custom"
	customPolicy := &customPolicy{}
	RegisterPolicy(name, func() Policy { return customPolicy })
	if _, ok := supportedPolicies[name]; !ok {
		t.Error("Expected supportedPolicies to have a custom policy.")
	}

}

func TestAllowedPaths(t *testing.T) {
	upstream := &staticUpstream{
		from:              "miek.nl.",
		IgnoredSubDomains: []string{"download.", "static."}, // closing dot mandatory
	}
	tests := []struct {
		name     string
		expected bool
	}{
		{"miek.nl.", true},
		{"download.miek.nl.", false},
		{"static.miek.nl.", false},
		{"blaat.miek.nl.", true},
	}

	for i, test := range tests {
		isAllowed := upstream.IsAllowedPath(test.name)
		if test.expected != isAllowed {
			t.Errorf("Test %d: expected %v found %v for %s", i+1, test.expected, isAllowed, test.name)
		}
	}
}

func writeTmpFile(t *testing.T, data string) (string, string) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatalf("tempDir: %v", err)
	}

	path := filepath.Join(tempDir, "resolv.conf")
	if err := ioutil.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	return tempDir, path
}

func TestProxyParse(t *testing.T) {
	tests := []struct {
		inputUpstreams string
		shouldErr      bool
	}{
		{
			`proxy . 8.8.8.8:53`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
    policy round_robin
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
    fail_timeout 5s
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
    max_fails 10
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
    health_check /health:8080
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
    without without
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
    except miek.nl example.org
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
    spray
}`,
			false,
		},
		{
			`
proxy . 8.8.8.8:53 {
    error_option
}`,
			true,
		},
		{
			`
proxy . some_bogus_filename`,
			true,
		},
	}
	for i, test := range tests {
		c := caddy.NewTestController("dns", test.inputUpstreams)
		_, err := NewStaticUpstreams(&c.Dispenser)
		if (err != nil) != test.shouldErr {
			t.Errorf("Test %d expected no error, got %v", i+1, err)
		}
	}
}

func TestResolvParse(t *testing.T) {
	tests := []struct {
		inputUpstreams string
		filedata       string
		shouldErr      bool
		expected       []string
	}{
		{
			`
proxy . FILE
`,
			`
nameserver 1.2.3.4
nameserver 4.3.2.1
`,
			false,
			[]string{"1.2.3.4:53", "4.3.2.1:53"},
		},
		{
			`
proxy example.com 1.1.1.1:5000
proxy . FILE
proxy example.org 2.2.2.2:1234
`,
			`
nameserver 1.2.3.4
`,
			false,
			[]string{"1.1.1.1:5000", "1.2.3.4:53", "2.2.2.2:1234"},
		},
		{
			`
proxy example.com 1.1.1.1:5000
proxy . FILE
proxy example.org 2.2.2.2:1234
`,
			`
junky resolve.conf
`,
			false,
			[]string{"1.1.1.1:5000", "2.2.2.2:1234"},
		},
	}
	for i, test := range tests {
		tempDir, path := writeTmpFile(t, test.filedata)
		defer os.RemoveAll(tempDir)
		config := strings.Replace(test.inputUpstreams, "FILE", path, -1)
		c := caddy.NewTestController("dns", config)
		upstreams, err := NewStaticUpstreams(&c.Dispenser)
		if (err != nil) != test.shouldErr {
			t.Errorf("Test %d expected no error, got %v", i+1, err)
		}
		var hosts []string
		for _, u := range upstreams {
			for _, h := range u.(*staticUpstream).Hosts {
				hosts = append(hosts, h.Name)
			}
		}
		if !test.shouldErr {
			if len(hosts) != len(test.expected) {
				t.Errorf("Test %d expected %d hosts got %d", i+1, len(test.expected), len(upstreams))
			} else {
				ok := true
				for i, v := range test.expected {
					if v != hosts[i] {
						ok = false
					}
				}
				if !ok {
					t.Errorf("Test %d expected %v got %v", i+1, test.expected, upstreams)
				}
			}
		}
	}
}
