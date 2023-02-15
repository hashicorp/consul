package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPI_ConfigEntries_InlineCertificate(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	configEntries := c.ConfigEntries()

	cert1 := &InlineCertificateConfigEntry{
		Kind: InlineCertificate,
		Name: "cert1",
		Meta: map[string]string{"foo": "bar"},
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
	cert1.Certificate = "certificate"
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

	// delete it
	wm, err = configEntries.Delete(InlineCertificate, cert1.Name, nil)
	require.NoError(t, err)
	require.NotNil(t, wm)
	assert.NotEqual(t, 0, wm.RequestTime)

	// try to get it
	_, _, err = configEntries.Get(InlineCertificate, cert1.Name, nil)
	assert.Error(t, err)
}
