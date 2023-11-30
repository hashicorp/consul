// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// generated via openssl req -x509 -sha256 -days 1825 -newkey rsa:2048 -keyout private.key -out certificate.crt
	validPrivateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0wzZeonUklhOvJ0AxcdDdCTiMwR9tsm/6IGcw9Jm50xVY+qg
5GFg1RWrQaODq7Gjqd/JDUAwtTBnQMs1yt6nbsHe2QhbD4XeqtZ+6fTv1ZpG3k8F
eB/M01xFqovczRV/ie77wd4vqoPD+AcfD8NDAFJt3htwUgGIqkQHP329Sh3TtLga
9ZMCs1MoTT+POYGUPL8bwt9R6ClNrucbH4Bs6OnX2ZFbKF75O9OHKNxWTmpDSodv
OFbFyKps3BfnPuF0Z6mj5M5yZeCjmtfS25PrsM3pMBGK5YHb0MlFfZIrIGboMbrz
9F/BMQJ64pMe43KwqHvTnbKWhp6PzLhEkPGLnwIDAQABAoIBADBEJAiONPszDu67
yU1yAM8zEDgysr127liyK7PtDnOfVXgAVMNmMcsJpZzhVF+TxKY487YAFCOb6kE7
OBYpTYla9SgVbR3js8TGQUgoKCFlowd8cvfB7gn4dEZIrjqIzB4zdYgk1Cne8JZs
qoHkWhJcx5ugEtPuXd7yp+WxT/T+6uOro06scp67NhP5t9yoAGFv5Vdb577RuzRo
Wkd9higQ9A20+GtjCY0EYxdgRviWvW7mM5/F+Lzcaui86ME+ga754gX8zgW3+NJ5
LMsz5OLSnh291Uyjmr77HWBv/xvpq01Fls0LyJcgxFVZuJs5GQz+l3otSqv4FTP6
Ua9w/YECgYEA8To3dgUK1QhzX5rwhWtlst3pItGTvmEdNzXmjgSylu7uKM13i+xg
llhp2uXrOEtuL+xtBZdeFNaijusbyqjg0xj6e4o31c19okuuDkJD5/sfQq22bvrn
gVJMGuESprIiPePrEyrXCHOdxH6eDgR2dIzAeO5vz0nnKGFAWrJJbvECgYEA3/mJ
eacXOJznw4Sa8jGWS2FtZLKxDHph7uDKMJmuG0ukb3aHJ9dMHrPleCLo8mhpoObA
hueoIbIP7swGrQx79+nZbnQpF6rMp6FAU5bF3gSrj1eWbaeh8pn9mrv4hal9USmn
orTbXMxDp3XSh7voR8Fqy5tMQqwZ+Lz74ccbw48CgYEA5cEhGdNrocPOv3x/IVRN
JLOfXX5nTaiJfxBja1imEIO5ajtoZWjaBdhn2gmqo4+UfyicHfsxrH9RjPX5HmkC
2Yys5gWbcJOr2Wxjd0k+DDFucL+rRsDKxq1vtxov/X0kh/YQ68ydynr0BTbjq04s
1I1KtOPEspYdCKS3+qpcrsECgYBtvYeVesBO9do9G0kMKC26y4bdEwzaz1ASykNn
IrWDHEH6dznr1HqwhHaHsZsvwucWdlmZAAKKWAOkfoU63uYS55qomvPTa9WQwNqS
2koi6Wjh+Al1uvAHvVncKgOwAgar8Nv5ReJBirgPYhSAexpppiRclL/93vNuw7Iq
wvMgkwKBgQC5wnb6SUUrzzKKSRgyusHM/XrjiKgVKq7lvFE9/iJkcw+BEXpjjbEe
RyD0a7PRtCfR39SMVrZp4KXVNNK5ln0WhuLvraMDwOpH9JDWHQiAhuJ3ooSwBylK
+QCLjyOtWAGZAIBRJyb1txfTXZ++dldkOjBi3bmEiadOa48ksvDsNQ==
-----END RSA PRIVATE KEY-----`
	validCertificate = `-----BEGIN CERTIFICATE-----
MIIDQjCCAioCCQC6cMRYsE+ahDANBgkqhkiG9w0BAQsFADBjMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCQ0ExCzAJBgNVBAcMAkxBMQ0wCwYDVQQKDARUZXN0MQ0wCwYD
VQQLDARTdHViMRwwGgYDVQQDDBNob3N0LmNvbnN1bC5leGFtcGxlMB4XDTIzMDIx
NzAyMTA1MloXDTI4MDIxNjAyMTA1MlowYzELMAkGA1UEBhMCVVMxCzAJBgNVBAgM
AkNBMQswCQYDVQQHDAJMQTENMAsGA1UECgwEVGVzdDENMAsGA1UECwwEU3R1YjEc
MBoGA1UEAwwTaG9zdC5jb25zdWwuZXhhbXBsZTCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBANMM2XqJ1JJYTrydAMXHQ3Qk4jMEfbbJv+iBnMPSZudMVWPq
oORhYNUVq0Gjg6uxo6nfyQ1AMLUwZ0DLNcrep27B3tkIWw+F3qrWfun079WaRt5P
BXgfzNNcRaqL3M0Vf4nu+8HeL6qDw/gHHw/DQwBSbd4bcFIBiKpEBz99vUod07S4
GvWTArNTKE0/jzmBlDy/G8LfUegpTa7nGx+AbOjp19mRWyhe+TvThyjcVk5qQ0qH
bzhWxciqbNwX5z7hdGepo+TOcmXgo5rX0tuT67DN6TARiuWB29DJRX2SKyBm6DG6
8/RfwTECeuKTHuNysKh7052yloaej8y4RJDxi58CAwEAATANBgkqhkiG9w0BAQsF
AAOCAQEAHF10odRNJ7TKvcD2JPtR8wMacfldSiPcQnn+rhMUyBaKOoSrALxOev+N
L8N+RtEV+KXkyBkvT71OZzEpY9ROwqOQ/acnMdbfG0IBPbg3c/7WDD2sjcdr1zvc
U3T7WJ7G3guZ5aWCuAGgOyT6ZW8nrDa4yFbKZ1PCJkvUQ2ttO1lXmyGPM533Y2pi
SeXP6LL7z5VNqYO3oz5IJEstt10IKxdmb2gKFhHjgEmHN2gFL0jaPi4mjjaINrxq
MdqcM9IzLr26AjZ45NuI9BCcZWO1mraaQTOIb3QL5LyqaC7CRJXLYPSGARthyDhq
J3TrQE3YVrL4D9xnklT86WDnZKApJg==
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
