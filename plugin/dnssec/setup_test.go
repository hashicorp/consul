package dnssec

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/caddyserver/caddy"
)

func TestSetupDnssec(t *testing.T) {
	if err := ioutil.WriteFile("Kcluster.local.key", []byte(keypub), 0644); err != nil {
		t.Fatalf("Failed to write pub key file: %s", err)
	}
	defer func() { os.Remove("Kcluster.local.key") }()
	if err := ioutil.WriteFile("Kcluster.local.private", []byte(keypriv), 0644); err != nil {
		t.Fatalf("Failed to write private key file: %s", err)
	}
	defer func() { os.Remove("Kcluster.local.private") }()
	if err := ioutil.WriteFile("ksk_Kcluster.local.key", []byte(kskpub), 0644); err != nil {
		t.Fatalf("Failed to write pub key file: %s", err)
	}
	defer func() { os.Remove("ksk_Kcluster.local.key") }()
	if err := ioutil.WriteFile("ksk_Kcluster.local.private", []byte(kskpriv), 0644); err != nil {
		t.Fatalf("Failed to write private key file: %s", err)
	}
	defer func() { os.Remove("ksk_Kcluster.local.private") }()

	tests := []struct {
		input              string
		shouldErr          bool
		expectedZones      []string
		expectedKeys       []string
		expectedSplitkeys  bool
		expectedCapacity   int
		expectedErrContent string
	}{
		{`dnssec`, false, nil, nil, false, defaultCap, ""},
		{`dnssec example.org`, false, []string{"example.org."}, nil, false, defaultCap, ""},
		{`dnssec 10.0.0.0/8`, false, []string{"10.in-addr.arpa."}, nil, false, defaultCap, ""},
		{
			`dnssec example.org {
				cache_capacity 100
			}`, false, []string{"example.org."}, nil, false, 100, "",
		},
		{
			`dnssec cluster.local {
				key file Kcluster.local
			}`, false, []string{"cluster.local."}, nil, false, defaultCap, "",
		},
		{
			`dnssec example.org cluster.local {
				key file Kcluster.local
			}`, false, []string{"example.org.", "cluster.local."}, nil, false, defaultCap, "",
		},
		// fails
		{
			`dnssec example.org {
				key file Kcluster.local
			}`, true, []string{"example.org."}, nil, false, defaultCap, "can not sign any",
		},
		{
			`dnssec example.org {
				key
			}`, true, []string{"example.org."}, nil, false, defaultCap, "argument count",
		},
		{
			`dnssec example.org {
				key file
			}`, true, []string{"example.org."}, nil, false, defaultCap, "argument count",
		},
		{`dnssec
		  dnssec`, true, nil, nil, false, defaultCap, ""},
		{
			`dnssec cluster.local {
				key file Kcluster.local
				key file ksk_Kcluster.local
			}`, false, []string{"cluster.local."}, nil, true, defaultCap, "",
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		zones, keys, capacity, splitkeys, err := dnssecParse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected error but found %s for input %s", i, err, test.input)
		}

		if err != nil {
			if !test.shouldErr {
				t.Errorf("Test %d: Expected no error but found one for input %s. Error was: %v", i, test.input, err)
			}

			if !strings.Contains(err.Error(), test.expectedErrContent) {
				t.Errorf("Test %d: Expected error to contain: %v, found error: %v, input: %s", i, test.expectedErrContent, err, test.input)
			}
		}
		if !test.shouldErr {
			for i, z := range test.expectedZones {
				if zones[i] != z {
					t.Errorf("Dnssec not correctly set for input %s. Expected: %s, actual: %s", test.input, z, zones[i])
				}
			}
			for i, k := range test.expectedKeys {
				if k != keys[i].K.Header().Name {
					t.Errorf("Dnssec not correctly set for input %s. Expected: '%s', actual: '%s'", test.input, k, keys[i].K.Header().Name)
				}
			}
			if splitkeys != test.expectedSplitkeys {
				t.Errorf("Detected split keys does not match. Expected: %t, actual %t", test.expectedSplitkeys, splitkeys)
			}
			if capacity != test.expectedCapacity {
				t.Errorf("Dnssec not correctly set capacity for input '%s' Expected: '%d', actual: '%d'", test.input, capacity, test.expectedCapacity)
			}
		}
	}
}

const keypub = `; This is a zone-signing key, keyid 45330, for cluster.local.
; Created: 20170901060531 (Fri Sep  1 08:05:31 2017)
; Publish: 20170901060531 (Fri Sep  1 08:05:31 2017)
; Activate: 20170901060531 (Fri Sep  1 08:05:31 2017)
cluster.local. IN DNSKEY 256 3 5 AwEAAcFpDv+Cb23kFJowu+VU++b2N1uEHi6Ll9H0BzLasFOdJjEEclCO q/KlD4682vOMXxJNN8ZwOyiCa7Y0TEYqSwWvhHyn3bHCwuy4I6fss4Wd 7Y9dU+6QTgJ8LimGG40Iizjc9zqoU8Q+q81vIukpYWOHioHoY7hsWBvS RSlzDJk3`

const keypriv = `Private-key-format: v1.3
Algorithm: 5 (RSASHA1)
Modulus: wWkO/4JvbeQUmjC75VT75vY3W4QeLouX0fQHMtqwU50mMQRyUI6r8qUPjrza84xfEk03xnA7KIJrtjRMRipLBa+EfKfdscLC7Lgjp+yzhZ3tj11T7pBOAnwuKYYbjQiLONz3OqhTxD6rzW8i6SlhY4eKgehjuGxYG9JFKXMMmTc=
PublicExponent: AQAB
PrivateExponent: K5XyZFBPrjMVFX5gCZlyPyVDamNGrfSVXSIiMSqpS96BSdCXtmHAjCj4bZFPwkzi6+vs4tJN8p4ZifEVM0a6qwPZyENBrc2qbsweOXE6l8BaPVWFX30xvVRzGXuNtXxlBXE17zoHty5r5mRyRou1bc2HUS5otdkEjE30RiocQVk=
Prime1: 7RRFUxaZkVNVH1DaT/SV5Sb8kABB389qLwU++argeDCVf+Wm9BBlTrsz2U6bKlfpaUmYZKtCCd+CVxqzMyuu0w==
Prime2: 0NiY3d7Fa08IGY9L4TaFc02A721YcDNBBf95BP31qGvwnYsLFM/1xZwaEsIjohg8g+m/GpyIlvNMbK6pywIVjQ==
Exponent1: XjXO8pype9mMmvwrNNix9DTQ6nxfsQugW30PMHGZ78kGr6NX++bEC0xS50jYWjRDGcbYGzD+9iNujSScD3qNZw==
Exponent2: wkoOhLIfhUIj7etikyUup2Ld5WAbW15DSrotstg0NrgcQ+Q7reP96BXeJ79WeREFE09cyvv/EjdLzPv81/CbbQ==
Coefficient: ah4LL0KLTO8kSKHK+X9Ud8grYi94QSNdbX11ge/eFcS/41QhDuZRTAFv4y0+IG+VWd+XzojLsQs+jzLe5GzINg==
Created: 20170901060531
Publish: 20170901060531
Activate: 20170901060531
`

const kskpub = `; This is a zone-signing key, keyid 45330, for cluster.local.
; Created: 20170901060531 (Fri Sep  1 08:05:31 2017)
; Publish: 20170901060531 (Fri Sep  1 08:05:31 2017)
; Activate: 20170901060531 (Fri Sep  1 08:05:31 2017)
cluster.local. IN DNSKEY 257 3 5 AwEAAcFpDv+Cb23kFJowu+VU++b2N1uEHi6Ll9H0BzLasFOdJjEEclCO q/KlD4682vOMXxJNN8ZwOyiCa7Y0TEYqSwWvhHyn3bHCwuy4I6fss4Wd 7Y9dU+6QTgJ8LimGG40Iizjc9zqoU8Q+q81vIukpYWOHioHoY7hsWBvS RSlzDJk3`

const kskpriv = `Private-key-format: v1.3
Algorithm: 5 (RSASHA1)
Modulus: wWkO/4JvbeQUmjC75VT75vY3W4QeLouX0fQHMtqwU50mMQRyUI6r8qUPjrza84xfEk03xnA7KIJrtjRMRipLBa+EfKfdscLC7Lgjp+yzhZ3tj11T7pBOAnwuKYYbjQiLONz3OqhTxD6rzW8i6SlhY4eKgehjuGxYG9JFKXMMmTc=
PublicExponent: AQAB
PrivateExponent: K5XyZFBPrjMVFX5gCZlyPyVDamNGrfSVXSIiMSqpS96BSdCXtmHAjCj4bZFPwkzi6+vs4tJN8p4ZifEVM0a6qwPZyENBrc2qbsweOXE6l8BaPVWFX30xvVRzGXuNtXxlBXE17zoHty5r5mRyRou1bc2HUS5otdkEjE30RiocQVk=
Prime1: 7RRFUxaZkVNVH1DaT/SV5Sb8kABB389qLwU++argeDCVf+Wm9BBlTrsz2U6bKlfpaUmYZKtCCd+CVxqzMyuu0w==
Prime2: 0NiY3d7Fa08IGY9L4TaFc02A721YcDNBBf95BP31qGvwnYsLFM/1xZwaEsIjohg8g+m/GpyIlvNMbK6pywIVjQ==
Exponent1: XjXO8pype9mMmvwrNNix9DTQ6nxfsQugW30PMHGZ78kGr6NX++bEC0xS50jYWjRDGcbYGzD+9iNujSScD3qNZw==
Exponent2: wkoOhLIfhUIj7etikyUup2Ld5WAbW15DSrotstg0NrgcQ+Q7reP96BXeJ79WeREFE09cyvv/EjdLzPv81/CbbQ==
Coefficient: ah4LL0KLTO8kSKHK+X9Ud8grYi94QSNdbX11ge/eFcS/41QhDuZRTAFv4y0+IG+VWd+XzojLsQs+jzLe5GzINg==
Created: 20170901060531
Publish: 20170901060531
Activate: 20170901060531
`
