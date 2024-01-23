// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/tlsutil"
	"github.com/hashicorp/go-uuid"
	"github.com/mitchellh/cli"
	"github.com/stretchr/testify/require"
)

func Test_loadPersistedBootstrapConfig(t *testing.T) {
	type expect struct {
		loaded  bool
		warning string
	}
	type testCase struct {
		existingCluster        bool
		disableManagementToken bool
		mutateFn               func(t *testing.T, dir string)
		expect                 expect
	}

	run := func(t *testing.T, tc testCase) {
		dataDir, err := os.MkdirTemp(os.TempDir(), "load-bootstrap-test-")
		require.NoError(t, err)
		t.Cleanup(func() { os.RemoveAll(dataDir) })

		dir := filepath.Join(dataDir, SubDir)

		// Do some common setup as if we received config from HCP and persisted it to disk.
		require.NoError(t, lib.EnsurePath(dir, true))
		require.NoError(t, persistSuccessMarker(dir))

		if !tc.existingCluster {
			caCert, caKey, err := tlsutil.GenerateCA(tlsutil.CAOpts{})
			require.NoError(t, err)

			serverCert, serverKey, err := testLeaf(caCert, caKey)
			require.NoError(t, err)
			require.NoError(t, persistTLSCerts(dir, serverCert, serverKey, []string{caCert}))

			cfgJSON := `{"bootstrap_expect": 8}`
			require.NoError(t, persistBootstrapConfig(dir, cfgJSON))
		}

		var token string
		if !tc.disableManagementToken {
			token, err = uuid.GenerateUUID()
			require.NoError(t, err)
			require.NoError(t, persistManagementToken(dir, token))
		}

		// Optionally mutate the persisted data to trigger errors while loading.
		if tc.mutateFn != nil {
			tc.mutateFn(t, dir)
		}

		ui := cli.NewMockUi()
		cfg, loaded := LoadPersistedBootstrapConfig(dataDir, ui)
		require.Equal(t, tc.expect.loaded, loaded, ui.ErrorWriter.String())
		if loaded {
			require.Equal(t, token, cfg.ManagementToken)
			require.Empty(t, ui.ErrorWriter.String())
		} else {
			require.Nil(t, cfg)
			require.Contains(t, ui.ErrorWriter.String(), tc.expect.warning)
		}
	}

	tt := map[string]testCase{
		"existing cluster with valid files": {
			existingCluster: true,
			// Don't mutate, files from setup are valid.
			mutateFn: nil,
			expect: expect{
				loaded:  true,
				warning: "",
			},
		},
		"existing cluster no token": {
			existingCluster:        true,
			disableManagementToken: true,
			expect: expect{
				loaded: false,
			},
		},
		"existing cluster no files": {
			existingCluster: true,
			mutateFn: func(t *testing.T, dir string) {
				// Remove all files
				require.NoError(t, os.RemoveAll(dir))
			},
			expect: expect{
				loaded: false,
				// No warnings since we assume we need to fetch config from HCP for the first time.
				warning: "",
			},
		},
		"new cluster with valid files": {
			// Don't mutate, files from setup are valid.
			mutateFn: nil,
			expect: expect{
				loaded:  true,
				warning: "",
			},
		},
		"new cluster with no token": {
			disableManagementToken: true,
			expect: expect{
				loaded: false,
			},
		},
		"new cluster some files": {
			mutateFn: func(t *testing.T, dir string) {
				// Remove one of the required files
				require.NoError(t, os.Remove(filepath.Join(dir, CertFileName)))
			},
			expect: expect{
				loaded:  false,
				warning: "configuration files on disk are incomplete",
			},
		},
		"new cluster no files": {
			mutateFn: func(t *testing.T, dir string) {
				// Remove all files
				require.NoError(t, os.RemoveAll(dir))
			},
			expect: expect{
				loaded: false,
				// No warnings since we assume we need to fetch config from HCP for the first time.
				warning: "",
			},
		},
		"new cluster invalid cert": {
			mutateFn: func(t *testing.T, dir string) {
				name := filepath.Join(dir, CertFileName)
				require.NoError(t, os.WriteFile(name, []byte("not-a-cert"), 0600))
			},
			expect: expect{
				loaded:  false,
				warning: "invalid server certificate",
			},
		},
		"new cluster invalid CA": {
			mutateFn: func(t *testing.T, dir string) {
				name := filepath.Join(dir, CAFileName)
				require.NoError(t, os.WriteFile(name, []byte("not-a-ca-cert"), 0600))
			},
			expect: expect{
				loaded:  false,
				warning: "invalid CA certificate",
			},
		},
		"existing cluster invalid token": {
			existingCluster: true,
			mutateFn: func(t *testing.T, dir string) {
				name := filepath.Join(dir, TokenFileName)
				require.NoError(t, os.WriteFile(name, []byte("not-a-uuid"), 0600))
			},
			expect: expect{
				loaded:  false,
				warning: "is not a valid UUID",
			},
		},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			run(t, tc)
		})
	}
}
