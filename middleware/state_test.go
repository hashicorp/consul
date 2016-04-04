package middleware

import (
	"testing"

	coretest "github.com/miekg/coredns/middleware/testing"

	"github.com/miekg/dns"
)

func TestStateDo(t *testing.T) {
	st := testState()

	st.Do()
	if st.do == 0 {
		t.Fatalf("expected st.do to be set")
	}
}

func BenchmarkStateDo(b *testing.B) {
	st := testState()

	for i := 0; i < b.N; i++ {
		st.Do()
	}
}

func BenchmarkStateSize(b *testing.B) {
	st := testState()

	for i := 0; i < b.N; i++ {
		st.Size()
	}
}

func testState() State {
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)
	m.SetEdns0(4097, true)

	return State{W: &coretest.ResponseWriter{}, Req: m}
}

/*
func TestHeader(t *testing.T) {
	state := getContextOrFail(t)

	headerKey, headerVal := "Header1", "HeaderVal1"
	state.Req.Header.Add(headerKey, headerVal)

	actualHeaderVal := state.Header(headerKey)
	if actualHeaderVal != headerVal {
		t.Errorf("Expected header %s, found %s", headerVal, actualHeaderVal)
	}

	missingHeaderVal := state.Header("not-existing")
	if missingHeaderVal != "" {
		t.Errorf("Expected empty header value, found %s", missingHeaderVal)
	}
}

func TestIP(t *testing.T) {
	state := getContextOrFail(t)

	tests := []struct {
		inputRemoteAddr string
		expectedIP      string
	}{
		// Test 0 - ipv4 with port
		{"1.1.1.1:1111", "1.1.1.1"},
		// Test 1 - ipv4 without port
		{"1.1.1.1", "1.1.1.1"},
		// Test 2 - ipv6 with port
		{"[::1]:11", "::1"},
		// Test 3 - ipv6 without port and brackets
		{"[2001:db8:a0b:12f0::1]", "[2001:db8:a0b:12f0::1]"},
		// Test 4 - ipv6 with zone and port
		{`[fe80:1::3%eth0]:44`, `fe80:1::3%eth0`},
	}

	for i, test := range tests {
		testPrefix := getTestPrefix(i)

		state.Req.RemoteAddr = test.inputRemoteAddr
		actualIP := state.IP()

		if actualIP != test.expectedIP {
			t.Errorf(testPrefix+"Expected IP %s, found %s", test.expectedIP, actualIP)
		}
	}
}

func TestURL(t *testing.T) {
	state := getContextOrFail(t)

	inputURL := "http://localhost"
	state.Req.RequestURI = inputURL

	if inputURL != state.URI() {
		t.Errorf("Expected url %s, found %s", inputURL, state.URI())
	}
}

func TestHost(t *testing.T) {
	tests := []struct {
		input        string
		expectedHost string
		shouldErr    bool
	}{
		{
			input:        "localhost:123",
			expectedHost: "localhost",
			shouldErr:    false,
		},
		{
			input:        "localhost",
			expectedHost: "localhost",
			shouldErr:    false,
		},
		{
			input:        "[::]",
			expectedHost: "",
			shouldErr:    true,
		},
	}

	for _, test := range tests {
		testHostOrPort(t, true, test.input, test.expectedHost, test.shouldErr)
	}
}

func TestPort(t *testing.T) {
	tests := []struct {
		input        string
		expectedPort string
		shouldErr    bool
	}{
		{
			input:        "localhost:123",
			expectedPort: "123",
			shouldErr:    false,
		},
		{
			input:        "localhost",
			expectedPort: "80", // assuming 80 is the default port
			shouldErr:    false,
		},
		{
			input:        ":8080",
			expectedPort: "8080",
			shouldErr:    false,
		},
		{
			input:        "[::]",
			expectedPort: "",
			shouldErr:    true,
		},
	}

	for _, test := range tests {
		testHostOrPort(t, false, test.input, test.expectedPort, test.shouldErr)
	}
}

func testHostOrPort(t *testing.T, isTestingHost bool, input, expectedResult string, shouldErr bool) {
	state := getContextOrFail(t)

	state.Req.Host = input
	var actualResult, testedObject string
	var err error

	if isTestingHost {
		actualResult, err = state.Host()
		testedObject = "host"
	} else {
		actualResult, err = state.Port()
		testedObject = "port"
	}

	if shouldErr && err == nil {
		t.Errorf("Expected error, found nil!")
		return
	}

	if !shouldErr && err != nil {
		t.Errorf("Expected no error, found %s", err)
		return
	}

	if actualResult != expectedResult {
		t.Errorf("Expected %s %s, found %s", testedObject, expectedResult, actualResult)
	}
}

func TestPathMatches(t *testing.T) {
	state := getContextOrFail(t)

	tests := []struct {
		urlStr      string
		pattern     string
		shouldMatch bool
	}{
		// Test 0
		{
			urlStr:      "http://localhost/",
			pattern:     "",
			shouldMatch: true,
		},
		// Test 1
		{
			urlStr:      "http://localhost",
			pattern:     "",
			shouldMatch: true,
		},
		// Test 1
		{
			urlStr:      "http://localhost/",
			pattern:     "/",
			shouldMatch: true,
		},
		// Test 3
		{
			urlStr:      "http://localhost/?param=val",
			pattern:     "/",
			shouldMatch: true,
		},
		// Test 4
		{
			urlStr:      "http://localhost/dir1/dir2",
			pattern:     "/dir2",
			shouldMatch: false,
		},
		// Test 5
		{
			urlStr:      "http://localhost/dir1/dir2",
			pattern:     "/dir1",
			shouldMatch: true,
		},
		// Test 6
		{
			urlStr:      "http://localhost:444/dir1/dir2",
			pattern:     "/dir1",
			shouldMatch: true,
		},
	}

	for i, test := range tests {
		testPrefix := getTestPrefix(i)
		var err error
		state.Req.URL, err = url.Parse(test.urlStr)
		if err != nil {
			t.Fatalf("Failed to prepare test URL from string %s! Error was: %s", test.urlStr, err)
		}

		matches := state.PathMatches(test.pattern)
		if matches != test.shouldMatch {
			t.Errorf(testPrefix+"Expected and actual result differ: expected to match [%t], actual matches [%t]", test.shouldMatch, matches)
		}
	}
}

func initTestContext() (Context, error) {
	body := bytes.NewBufferString("request body")
	request, err := http.NewRequest("GET", "https://localhost", body)
	if err != nil {
		return Context{}, err
	}

	return Context{Root: http.Dir(os.TempDir()), Req: request}, nil
}


func getTestPrefix(testN int) string {
	return fmt.Sprintf("Test [%d]: ", testN)
}
*/
