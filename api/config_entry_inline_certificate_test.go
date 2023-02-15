package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// generated via openssl req -x509 -sha256 -days 1825 -newkey rsa:2048 -keyout private.key -out certificate.crt
	validPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAx95Opa6t4lGEpiTUogEBptqOdam2ch4BHQGhNhX/MrDwwuZQ
httBwMfngQ/wd9NmYEPAwj0dumUoAITIq6i2jQlhqTodElkbsd5vWY8R/bxJWQSo
NvVE12TlzECxGpJEiHt4W0r8pGffk+rvpljiUyCfnT1kGF3znOSjK1hRMTn6RKWC
yYaBvXQiB4SGilfLgJcEpOJKtISIxmZ+S409g9X5VU88/Bmmrz4cMyxce86Kc2ug
5/MOv0CjWDJwlrv8njneV2zvraQ61DDwQftrXOvuCbO5IBRHMOBHiHTZ4rtGuhMa
Ir21V4vb6n8c4YzXiFvhUYcyX7rltGZzVd+WmQIDAQABAoIBACYvceUzp2MK4gYA
GWPOP2uKbBdM0l+hHeNV0WAM+dHMfmMuL4pkT36ucqt0ySOLjw6rQyOZG5nmA6t9
sv0g4ae2eCMlyDIeNi1Yavu4Wt6YX4cTXbQKThm83C6W2X9THKbauBbxD621bsDK
7PhiGPN60yPue7YwFQAPqqD4YaK+s22HFIzk9gwM/rkvAUNwRv7SyHMiFe4Igc1C
Eev7iHWzvj5Heoz6XfF+XNF9DU+TieSUAdjd56VyUb8XL4+uBTOhHwLiXvAmfaMR
HvpcxeKnYZusS6NaOxcUHiJnsLNWrxmJj9WEGgQzuLxcLjTe4vVmELVZD8t3QUKj
PAxu8tUCgYEA7KIWVn9dfVpokReorFym+J8FzLwSktP9RZYEMonJo00i8aii3K9s
u/aSwRWQSCzmON1ZcxZzWhwQF9usz6kGCk//9+4hlVW90GtNK0RD+j7sp4aT2JI8
9eLEjTG+xSXa7XWe98QncjjL9lu/yrRncSTxHs13q/XP198nn2aYuQ8CgYEA2Dnt
sRBzv0fFEvzzFv7G/5f85mouN38TUYvxNRTjBLCXl9DeKjDkOVZ2b6qlfQnYXIru
H+W+v+AZEb6fySXc8FRab7lkgTMrwE+aeI4rkW7asVwtclv01QJ5wMnyT84AgDD/
Dgt/RThFaHgtU9TW5GOZveL+l9fVPn7vKFdTJdcCgYEArJ99zjHxwJ1whNAOk1av
09UmRPm6TvRo4heTDk8oEoIWCNatoHI0z1YMLuENNSnT9Q280FFDayvnrY/qnD7A
kktT/sjwJOG8q8trKzIMqQS4XWm2dxoPcIyyOBJfCbEY6XuRsUuePxwh5qF942EB
yS9a2s6nC4Ix0lgPrqAIr48CgYBgS/Q6riwOXSU8nqCYdiEkBYlhCJrKpnJxF9T1
ofa0yPzKZP/8ZEfP7VzTwHjxJehQ1qLUW9pG08P2biH1UEKEWdzo8vT6wVJT1F/k
HtTycR8+a+Hlk2SHVRHqNUYQGpuIe8mrdJ1as4Pd0d/F/P0zO9Rlh+mAsGPM8HUM
T0+9gwKBgHDoerX7NTskg0H0t8O+iSMevdxpEWp34ZYa9gHiftTQGyrRgERCa7Gj
nZPAxKb2JoWyfnu3v7G5gZ8fhDFsiOxLbZv6UZJBbUIh1MjJISpXrForDrC2QNLX
kHrHfwBFDB3KMudhQknsJzEJKCL/KmFH6o0MvsoaT9yzEl3K+ah/
-----END RSA PRIVATE KEY-----`
	validCertificate = `-----BEGIN CERTIFICATE-----
MIICljCCAX4CCQCQMDsYO8FrPjANBgkqhkiG9w0BAQsFADANMQswCQYDVQQGEwJV
UzAeFw0yMjEyMjAxNzUwMjVaFw0yNzEyMTkxNzUwMjVaMA0xCzAJBgNVBAYTAlVT
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAx95Opa6t4lGEpiTUogEB
ptqOdam2ch4BHQGhNhX/MrDwwuZQhttBwMfngQ/wd9NmYEPAwj0dumUoAITIq6i2
jQlhqTodElkbsd5vWY8R/bxJWQSoNvVE12TlzECxGpJEiHt4W0r8pGffk+rvplji
UyCfnT1kGF3znOSjK1hRMTn6RKWCyYaBvXQiB4SGilfLgJcEpOJKtISIxmZ+S409
g9X5VU88/Bmmrz4cMyxce86Kc2ug5/MOv0CjWDJwlrv8njneV2zvraQ61DDwQftr
XOvuCbO5IBRHMOBHiHTZ4rtGuhMaIr21V4vb6n8c4YzXiFvhUYcyX7rltGZzVd+W
mQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQBfCqoUIdPf/HGSbOorPyZWbyizNtHJ
GL7x9cAeIYxpI5Y/WcO1o5v94lvrgm3FNfJoGKbV66+JxOge731FrfMpHplhar1Z
RahYIzNLRBTLrwadLAZkApUpZvB8qDK4knsTWFYujNsylCww2A6ajzIMFNU4GkUK
NtyHRuD+KYRmjXtyX1yHNqfGN3vOQmwavHq2R8wHYuBSc6LAHHV9vG+j0VsgMELO
qwxn8SmLkSKbf2+MsQVzLCXXN5u+D8Yv+4py+oKP4EQ5aFZuDEx+r/G/31rTthww
AAJAMaoXmoYVdgXV+CPuBb2M4XCpuzLu3bcA2PXm5ipSyIgntMKwXV7r
-----END CERTIFICATE-----`
)

func TestAPI_ConfigEntries_InlineCertificate(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	configEntries := c.ConfigEntries()

	cert1 := &InlineCertificateConfigEntry{
		Kind:        InlineCertificate,
		Name:        "cert1",
		Meta:        map[string]string{"foo": "bar"},
		Certificate: validCertificate,
		PrivateKey:  validPrivateKey,
	}

	// set it
	_, wm, err := configEntries.Set(cert1, nil)
	require.NoError(t, err)
	assert.NotNil(t, wm)

	// get it
	entry, qm, err := configEntries.Get(InlineCertificate, "cert1", nil)
	require.NoError(t, err)
	require.NotNil(t, qm)
	assert.NotEqual(t, 0, qm.RequestTime)

	readCert, ok := entry.(*InlineCertificateConfigEntry)
	require.True(t, ok)
	assert.Equal(t, cert1.Kind, readCert.Kind)
	assert.Equal(t, cert1.Name, readCert.Name)
	assert.Equal(t, cert1.Meta, readCert.Meta)
	assert.Equal(t, cert1.Meta, readCert.GetMeta())

	// update it
	cert1.Meta["bar"] = "baz"
	written, wm, err := configEntries.CAS(cert1, readCert.ModifyIndex, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	assert.NotEqual(t, 0, wm.RequestTime)
	assert.True(t, written)

	// list it
	entries, qm, err := configEntries.List(InlineCertificate, nil)
	require.NoError(t, err)
	require.NotNil(t, qm)
	assert.NotEqual(t, 0, qm.RequestTime)

	require.Len(t, entries, 1)
	assert.Equal(t, cert1.Kind, entries[0].GetKind())
	assert.Equal(t, cert1.Name, entries[0].GetName())

	readCert, ok = entries[0].(*InlineCertificateConfigEntry)
	require.True(t, ok)
	assert.Equal(t, cert1.Certificate, readCert.Certificate)
	assert.Equal(t, cert1.Meta, readCert.Meta)

	// delete it
	wm, err = configEntries.Delete(InlineCertificate, cert1.Name, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	assert.NotEqual(t, 0, wm.RequestTime)

	// try to get it
	_, _, err = configEntries.Get(InlineCertificate, cert1.Name, nil)
	assert.Error(t, err)
}
