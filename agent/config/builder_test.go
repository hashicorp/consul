package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Basically just testing that injection of the extra
	// source works.
	devMode := true
	builderOpts := BuilderOpts{
		// putting this in dev mode so that the config validates
		// without having to specify a data directory
		DevMode: &devMode,
	}

	cfg, warnings, err := Load(builderOpts, FileSource{
		Name:   "test",
		Format: "hcl",
		Data:   `node_name = "hobbiton"`,
	},
		FileSource{
			Name:   "overrides",
			Format: "json",
			Data:   `{"check_reap_interval": "1ms"}`,
		})

	require.NoError(t, err)
	require.Empty(t, warnings)
	require.NotNil(t, cfg)
	require.Equal(t, "hobbiton", cfg.NodeName)
	require.Equal(t, 1*time.Millisecond, cfg.CheckReapInterval)
}

func TestShouldParseFile(t *testing.T) {
	var testcases = []struct {
		filename     string
		configFormat string
		expected     bool
	}{
		{filename: "config.json", expected: true},
		{filename: "config.hcl", expected: true},
		{filename: "config", configFormat: "hcl", expected: true},
		{filename: "config.js", configFormat: "json", expected: true},
		{filename: "config.yaml", expected: false},
	}

	for _, tc := range testcases {
		name := fmt.Sprintf("filename=%s, format=%s", tc.filename, tc.configFormat)
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.expected, shouldParseFile(tc.filename, tc.configFormat))
		})
	}
}

func TestNewBuilder_PopulatesSourcesFromConfigFiles(t *testing.T) {
	paths := setupConfigFiles(t)

	b, err := NewBuilder(BuilderOpts{ConfigFiles: paths})
	require.NoError(t, err)

	expected := []Source{
		FileSource{Name: paths[0], Format: "hcl", Data: "content a"},
		FileSource{Name: paths[1], Format: "json", Data: "content b"},
		FileSource{Name: filepath.Join(paths[3], "a.hcl"), Format: "hcl", Data: "content a"},
		FileSource{Name: filepath.Join(paths[3], "b.json"), Format: "json", Data: "content b"},
	}
	require.Equal(t, expected, b.Sources)
	require.Len(t, b.Warnings, 2)
}

func TestNewBuilder_PopulatesSourcesFromConfigFiles_WithConfigFormat(t *testing.T) {
	paths := setupConfigFiles(t)

	b, err := NewBuilder(BuilderOpts{ConfigFiles: paths, ConfigFormat: "hcl"})
	require.NoError(t, err)

	expected := []Source{
		FileSource{Name: paths[0], Format: "hcl", Data: "content a"},
		FileSource{Name: paths[1], Format: "hcl", Data: "content b"},
		FileSource{Name: paths[2], Format: "hcl", Data: "content c"},
		FileSource{Name: filepath.Join(paths[3], "a.hcl"), Format: "hcl", Data: "content a"},
		FileSource{Name: filepath.Join(paths[3], "b.json"), Format: "hcl", Data: "content b"},
		FileSource{Name: filepath.Join(paths[3], "c.yaml"), Format: "hcl", Data: "content c"},
	}
	require.Equal(t, expected, b.Sources)
}

// TODO: this would be much nicer with gotest.tools/fs
func setupConfigFiles(t *testing.T) []string {
	t.Helper()
	path, err := ioutil.TempDir("", t.Name())
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(path) })

	subpath := filepath.Join(path, "sub")
	err = os.Mkdir(subpath, 0755)
	require.NoError(t, err)

	for _, dir := range []string{path, subpath} {
		err = ioutil.WriteFile(filepath.Join(dir, "a.hcl"), []byte("content a"), 0644)
		require.NoError(t, err)

		err = ioutil.WriteFile(filepath.Join(dir, "b.json"), []byte("content b"), 0644)
		require.NoError(t, err)

		err = ioutil.WriteFile(filepath.Join(dir, "c.yaml"), []byte("content c"), 0644)
		require.NoError(t, err)
	}
	return []string{
		filepath.Join(path, "a.hcl"),
		filepath.Join(path, "b.json"),
		filepath.Join(path, "c.yaml"),
		subpath,
	}
}
