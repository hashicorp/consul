package local

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
	"github.com/stretchr/testify/require"
)

func TestStringHash(t *testing.T) {
	t.Parallel()

	in := "hello world"
	expected := "5eb63bbbe01eeed093cb22bb8f5acdc3"

	if out := stringHash(in); out != expected {
		t.Fatalf("bad: %s", out)
	}
}

func TestService(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	defer os.RemoveAll(dir)

	state := NewPersistentState(log.New(os.Stderr, "", log.LstdFlags), dir)

	svc := &structs.NodeService{
		Service: "redis",
		ID:      "redis-1",
		Tags:    []string{"shard-1"},
		Port:    8888,
	}
	token := "e0d97212-c590-4bdb-ae67-de49bf705448"

	state.WriteService(PersistedService{
		Service: svc,
		Token:   token,
	})
	state.waitSync()

	loaded, err := state.LoadServices()
	require.Nil(t, err)

	require.Equal(t, []PersistedService{
		{
			Service: svc,
			Token:   token,
		},
	}, loaded)

	state.RemoveService(svc.ID)
	state.waitSync()

	loaded, err = state.LoadServices()
	require.Nil(t, err)

	require.Equal(t, []PersistedService{}, loaded)
}

func TestServiceCompat(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	defer os.RemoveAll(dir)

	state := NewPersistentState(log.New(os.Stderr, "", log.LstdFlags), dir)

	svc := &structs.NodeService{
		Service: "redis",
		ID:      "redis-1",
		Tags:    []string{"shard-1"},
		Port:    8888,
	}

	compatEnc, err := json.Marshal(svc)
	require.Nil(t, err)
	err = os.MkdirAll(filepath.Join(dir, servicesDir), 0700)
	require.Nil(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, servicesDir, stringHash(svc.ID)), compatEnc, 0600)
	require.Nil(t, err)

	loaded, err := state.LoadServices()
	require.Nil(t, err)

	require.Equal(t, []PersistedService{
		{
			Service: svc,
		},
	}, loaded)
}

func TestCheck(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	defer os.RemoveAll(dir)

	state := NewPersistentState(log.New(os.Stderr, "", log.LstdFlags), dir)

	chk := &structs.HealthCheck{
		CheckID: types.CheckID("check-1"),
		Name:    "check_http",
	}
	chkType := &structs.CheckType{
		CheckID: types.CheckID("check-1"),
		HTTP:    "http://127.0.0.1:3333/status",
	}
	token := "e0d97212-c590-4bdb-ae67-de49bf705448"

	state.WriteCheck(PersistedCheck{
		Check:   chk,
		ChkType: chkType,
		Token:   token,
	})
	state.waitSync()

	loaded, err := state.LoadChecks()
	require.Nil(t, err)

	require.Equal(t, []PersistedCheck{
		{
			Check:   chk,
			ChkType: chkType,
			Token:   token,
		},
	}, loaded)

	state.RemoveCheck(chk.CheckID)
	state.waitSync()

	loaded, err = state.LoadChecks()
	require.Nil(t, err)

	require.Equal(t, []PersistedCheck{}, loaded)
}

func TestProxy(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	defer os.RemoveAll(dir)

	state := NewPersistentState(log.New(os.Stderr, "", log.LstdFlags), dir)

	proxy := &structs.ConnectManagedProxy{
		ProxyService: &structs.NodeService{
			Service: "redis",
			ID:      "redis-1",
			Tags:    []string{"shard-1"},
			Port:    8888,
		},
	}
	token := "e0d97212-c590-4bdb-ae67-de49bf705448"

	state.WriteProxy(PersistedProxy{
		Proxy:      proxy,
		ProxyToken: token,
	})
	state.waitSync()

	loaded, err := state.LoadProxies()
	require.Nil(t, err)

	require.Equal(t, []PersistedProxy{
		{
			Proxy:      proxy,
			ProxyToken: token,
		},
	}, loaded)

	state.RemoveProxy(proxy.ProxyService.ID)
	state.waitSync()

	loaded, err = state.LoadProxies()
	require.Nil(t, err)

	require.Equal(t, []PersistedProxy{}, loaded)
}

func TestCheckState(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	defer os.RemoveAll(dir)

	state := NewPersistentState(log.New(os.Stderr, "", log.LstdFlags), dir)

	checkState := PersistedCheckState{
		CheckID: "check-1",
		Status:  api.HealthWarning,
		Output:  "Heyy",
	}

	state.WriteCheckState(checkState)
	state.waitSync()

	loaded, err := state.LoadCheckState(checkState.CheckID)
	require.Nil(t, err)

	require.Equal(t, &checkState, loaded)

	state.RemoveCheckState(checkState.CheckID)
	state.waitSync()

	loaded, err = state.LoadCheckState(checkState.CheckID)
	require.Nil(t, err)
	require.Nil(t, loaded)
}

func TestServiceCorrupted(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	defer os.RemoveAll(dir)

	state := NewPersistentState(log.New(os.Stderr, "", log.LstdFlags), dir)

	svc := &structs.NodeService{
		Service: "redis",
		ID:      "redis-1",
		Tags:    []string{"shard-1"},
		Port:    8888,
	}
	token := "e0d97212-c590-4bdb-ae67-de49bf705448"

	state.WriteService(PersistedService{
		Service: svc,
		Token:   token,
	})
	state.waitSync()

	// filename starts with a's to ensure is it the first read
	corruptedPath := filepath.Join(dir, servicesDir, "aaaaa-corrupted")
	err = ioutil.WriteFile(corruptedPath, []byte("__not_json__"), 0600)
	require.Nil(t, err)

	loaded, err := state.LoadServices()
	require.Nil(t, err)

	require.Equal(t, []PersistedService{
		{
			Service: svc,
			Token:   token,
		},
	}, loaded)

	_, err = os.Stat(corruptedPath)
	require.True(t, os.IsNotExist(err))
}

func TestCheckCorrupted(t *testing.T) {
	t.Parallel()

	dir, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	defer os.RemoveAll(dir)

	state := NewPersistentState(log.New(os.Stderr, "", log.LstdFlags), dir)

	chk := &structs.HealthCheck{
		CheckID: types.CheckID("check-1"),
		Name:    "check_http",
	}
	chkType := &structs.CheckType{
		CheckID: types.CheckID("check-1"),
		HTTP:    "http://127.0.0.1:3333/status",
	}
	token := "e0d97212-c590-4bdb-ae67-de49bf705448"

	state.WriteCheck(PersistedCheck{
		Check:   chk,
		ChkType: chkType,
		Token:   token,
	})
	state.waitSync()

	// filename starts with a's to ensure is it the first read
	corruptedPath := filepath.Join(dir, checksDir, "aaaaa-corrupted")
	err = ioutil.WriteFile(corruptedPath, []byte("__not_json__"), 0600)
	require.Nil(t, err)

	loaded, err := state.LoadChecks()
	require.Nil(t, err)

	require.Equal(t, []PersistedCheck{
		{
			Check:   chk,
			ChkType: chkType,
			Token:   token,
		},
	}, loaded)

	_, err = os.Stat(corruptedPath)
	require.True(t, os.IsNotExist(err))
}
