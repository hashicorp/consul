// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package kubeauth

import (
	"bytes"
	"context"
	"testing"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/authmethod"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

func TestStructs_ACLAuthMethod_Kubernetes_MsgpackEncodeDecode(t *testing.T) {
	in := &structs.ACLAuthMethod{
		Name:        "k8s",
		Type:        "kubernetes",
		Description: "k00b",
		Config: map[string]interface{}{
			"Host":              "https://kube.api.internal:8443",
			"CACert":            "<my garbage ca cert>",
			"ServiceAccountJWT": "my.fake.jwt",
		},
		RaftIndex: structs.RaftIndex{
			CreateIndex: 5,
			ModifyIndex: 99,
		},
	}

	expectConfig := &Config{
		Host:              "https://kube.api.internal:8443",
		CACert:            "<my garbage ca cert>",
		ServiceAccountJWT: "my.fake.jwt",
	}

	var (
		// This is the common configuration pre-1.7.0
		handle1 = structs.TestingOldPre1dot7MsgpackHandle
		// This is the common configuration post-1.7.0
		handle2 = structs.MsgpackHandle
	)

	decoderCase := func(t *testing.T, encHandle, decHandle *codec.MsgpackHandle) {
		t.Helper()

		var buf bytes.Buffer
		enc := codec.NewEncoder(&buf, encHandle)
		require.NoError(t, enc.Encode(in))

		out := &structs.ACLAuthMethod{}
		dec := codec.NewDecoder(&buf, decHandle)
		require.NoError(t, dec.Decode(out))

		var config Config
		require.NoError(t, authmethod.ParseConfig(in.Config, &config))

		out.Config = in.Config // no longer care about how this field decoded
		require.Equal(t, in, out)
		require.Equal(t, expectConfig, &config)
		// TODO: verify json?
	}

	t.Run("old encoder and old decoder", func(t *testing.T) {
		decoderCase(t, handle1, handle1)
	})
	t.Run("old encoder and new decoder", func(t *testing.T) {
		decoderCase(t, handle1, handle2)
	})
	t.Run("new encoder and old decoder", func(t *testing.T) {
		decoderCase(t, handle2, handle1)
	})
	t.Run("new encoder and new decoder", func(t *testing.T) {
		decoderCase(t, handle2, handle2)
	})
}

func TestNewIdentity(t *testing.T) {
	testSrv := StartTestAPIServer(t)
	defer testSrv.Stop()

	method := &structs.ACLAuthMethod{
		Name:        "test-k8s",
		Description: "k8s test",
		Type:        "kubernetes",
		Config: map[string]interface{}{
			"Host":              testSrv.Addr(),
			"CACert":            testSrv.CACert(),
			"ServiceAccountJWT": goodJWT_A,
		},
	}
	validator, err := NewValidator(method)
	require.NoError(t, err)

	id := validator.NewIdentity()
	authmethod.RequireIdentityMatch(t, id, map[string]string{
		"serviceaccount.namespace": "",
		"serviceaccount.name":      "",
		"serviceaccount.uid":       "",
	},
		`serviceaccount.namespace == ""`,
		`serviceaccount.name == ""`,
		`serviceaccount.uid == ""`,
	)
}

func TestValidateLogin(t *testing.T) {
	testSrv := StartTestAPIServer(t)
	defer testSrv.Stop()

	testSrv.AuthorizeJWT(goodJWT_A)
	testSrv.SetAllowedServiceAccount(
		"default",
		"demo",
		"76091af4-4b56-11e9-ac4b-708b11801cbe",
		"",
		goodJWT_B,
	)

	method := &structs.ACLAuthMethod{
		Name:        "test-k8s",
		Description: "k8s test",
		Type:        "kubernetes",
		Config: map[string]interface{}{
			"Host":              testSrv.Addr(),
			"CACert":            testSrv.CACert(),
			"ServiceAccountJWT": goodJWT_A,
		},
	}
	validator, err := NewValidator(method)
	require.NoError(t, err)

	t.Run("invalid bearer token", func(t *testing.T) {
		_, err := validator.ValidateLogin(context.Background(), "invalid")
		require.Error(t, err)
	})

	t.Run("valid bearer token", func(t *testing.T) {
		id, err := validator.ValidateLogin(context.Background(), goodJWT_B)
		require.NoError(t, err)

		authmethod.RequireIdentityMatch(t, id, map[string]string{
			"serviceaccount.namespace": "default",
			"serviceaccount.name":      "demo",
			"serviceaccount.uid":       "76091af4-4b56-11e9-ac4b-708b11801cbe",
		},
			`serviceaccount.namespace == default`,
			`serviceaccount.name == "demo"`,
			`serviceaccount.uid == "76091af4-4b56-11e9-ac4b-708b11801cbe"`,
		)
	})

	// annotate the account
	testSrv.SetAllowedServiceAccount(
		"default",
		"demo",
		"76091af4-4b56-11e9-ac4b-708b11801cbe",
		"alternate-name",
		goodJWT_B,
	)

	t.Run("valid bearer token with annotation", func(t *testing.T) {
		id, err := validator.ValidateLogin(context.Background(), goodJWT_B)
		require.NoError(t, err)

		authmethod.RequireIdentityMatch(t, id, map[string]string{
			"serviceaccount.namespace": "default",
			"serviceaccount.name":      "alternate-name",
			"serviceaccount.uid":       "76091af4-4b56-11e9-ac4b-708b11801cbe",
		},
			`serviceaccount.namespace == default`,
			`serviceaccount.name == "alternate-name"`,
			`serviceaccount.uid == "76091af4-4b56-11e9-ac4b-708b11801cbe"`,
		)
	})
}

func TestNewValidator(t *testing.T) {
	ca := connect.TestCA(t, nil)

	type AM = *structs.ACLAuthMethod

	makeAuthMethod := func(f func(method AM)) *structs.ACLAuthMethod {
		method := &structs.ACLAuthMethod{
			Name:        "test-k8s",
			Description: "k8s test",
			Type:        "kubernetes",
			Config: map[string]interface{}{
				"Host":              "https://abc:8443",
				"CACert":            ca.RootCert,
				"ServiceAccountJWT": goodJWT_A,
			},
		}
		if f != nil {
			f(method)
		}
		return method
	}

	for _, test := range []struct {
		name   string
		method *structs.ACLAuthMethod
		ok     bool
	}{
		// bad
		{"wrong type", makeAuthMethod(func(method AM) {
			method.Type = "invalid"
		}), false},
		{"extra config", makeAuthMethod(func(method AM) {
			method.Config["extra"] = "config"
		}), false},
		{"wrong type of config", makeAuthMethod(func(method AM) {
			method.Config["Host"] = []int{12345}
		}), false},
		{"missing host", makeAuthMethod(func(method AM) {
			delete(method.Config, "Host")
		}), false},
		{"missing ca cert", makeAuthMethod(func(method AM) {
			delete(method.Config, "CACert")
		}), false},
		{"invalid ca cert", makeAuthMethod(func(method AM) {
			method.Config["CACert"] = "invalid"
		}), false},
		{"invalid jwt", makeAuthMethod(func(method AM) {
			method.Config["ServiceAccountJWT"] = "invalid"
		}), false},
		{"garbage host", makeAuthMethod(func(method AM) {
			method.Config["Host"] = "://:12345"
		}), false},
		// good
		{"normal", makeAuthMethod(nil), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			v, err := NewValidator(test.method)
			if test.ok {
				require.NoError(t, err)
				require.NotNil(t, v)
			} else {
				require.NotNil(t, err)
				require.Nil(t, v)
			}
		})
	}
}

// 'default/admin'
const goodJWT_A = "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJkZWZhdWx0Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImFkbWluLXRva2VuLXFsejQyIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlLWFjY291bnQubmFtZSI6ImFkbWluIiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZXJ2aWNlLWFjY291bnQudWlkIjoiNzM4YmMyNTEtNjUzMi0xMWU5LWI2N2YtNDhlNmM4YjhlY2I1Iiwic3ViIjoic3lzdGVtOnNlcnZpY2VhY2NvdW50OmRlZmF1bHQ6YWRtaW4ifQ.ixMlnWrAG7NVuTTKu8cdcYfM7gweS3jlKaEsIBNGOVEjPE7rtXtgMkAwjQTdYR08_0QBjkgzy5fQC5ZNyglSwONJ-bPaXGvhoH1cTnRi1dz9H_63CfqOCvQP1sbdkMeRxNTGVAyWZT76rXoCUIfHP4LY2I8aab0KN9FTIcgZRF0XPTtT70UwGIrSmRpxW38zjiy2ymWL01cc5VWGhJqVysmWmYk3wNp0h5N57H_MOrz4apQR4pKaamzskzjLxO55gpbmZFC76qWuUdexAR7DT2fpbHLOw90atN_NlLMY-VrXyW3-Ei5EhYaVreMB9PSpKwkrA4jULITohV-sxpa1LA"

// 'default/demo'
const goodJWT_B = "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJkZWZhdWx0Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9zZWNyZXQubmFtZSI6ImRlbW8tdG9rZW4ta21iOW4iLCJrdWJlcm5ldGVzLmlvL3NlcnZpY2VhY2NvdW50L3NlcnZpY2UtYWNjb3VudC5uYW1lIjoiZGVtbyIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50LnVpZCI6Ijc2MDkxYWY0LTRiNTYtMTFlOS1hYzRiLTcwOGIxMTgwMWNiZSIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDpkZWZhdWx0OmRlbW8ifQ.ZiAHjijBAOsKdum0Aix6lgtkLkGo9_Tu87dWQ5Zfwnn3r2FejEWDAnftTft1MqqnMzivZ9Wyyki5ZjQRmTAtnMPJuHC-iivqY4Wh4S6QWCJ1SivBv5tMZR79t5t8mE7R1-OHwst46spru1pps9wt9jsA04d3LpV0eeKYgdPTVaQKklxTm397kIMUugA6yINIBQ3Rh8eQqBgNwEmL4iqyYubzHLVkGkoP9MJikFI05vfRiHtYr-piXz6JFDzXMQj9rW6xtMmrBSn79ChbyvC5nz-Nj2rJPnHsb_0rDUbmXY5PpnMhBpdSH-CbZ4j8jsiib6DtaGJhVZeEQ1GjsFAZwQ"
