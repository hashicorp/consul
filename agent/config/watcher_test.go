package config

import (
	"fmt"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func TestNewWatcher(t *testing.T) {
	w, err := New(func(event *WatcherEvent) error {
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	require.NoError(t, err)
	require.NotNil(t, w)
}

func TestWatcherAddRemoveExist(t *testing.T) {
	w, err := New(func(event *WatcherEvent) error {
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	require.NoError(t, err)
	file := testutil.TempFile(t, "temp_config")
	_, err = file.Write([]byte("test config"))
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)
	err = w.Add(file.Name())
	require.NoError(t, err)
	h, ok := w.configFiles[file.Name()]
	require.True(t, ok)
	require.Equal(t, "7465737420636f6e666967e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", h.hash)
	err = w.Remove(file.Name())
	require.NoError(t, err)
	_, ok = w.configFiles[file.Name()]
	require.False(t, ok)
}

func TestWatcherAddNotExist(t *testing.T) {
	w, err := New(func(event *WatcherEvent) error {
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	require.NoError(t, err)
	file := testutil.TempFile(t, "temp_config")
	filename := file.Name() + randomString(16)
	err = w.Add(filename)
	require.True(t, os.IsNotExist(err))
	_, ok := w.configFiles[filename]
	require.False(t, ok)
}

func TestEventWatcherWrite(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		watcherCh <- event
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	require.NoError(t, err)
	file := testutil.TempFile(t, "temp_config")
	_, err = file.WriteString("test config")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)

	err = w.Add(file.Name())
	require.NoError(t, err)
	w.Start()
	_, err = file.WriteString("test config 2")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)
	require.NoError(t, assertEvent(file.Name(), watcherCh))
}

func TestEventWatcherRead(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		watcherCh <- event
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	require.NoError(t, err)
	file := testutil.TempFile(t, "temp_config")
	_, err = file.WriteString("test config")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)

	err = w.Add(file.Name())
	require.NoError(t, err)
	w.Start()
	_, err = os.ReadFile(file.Name())
	require.NoError(t, err)
	require.Error(t, assertEvent(file.Name(), watcherCh), "timedout waiting for event")
}

func TestEventWatcherChmod(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		watcherCh <- event
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	require.NoError(t, err)
	file := testutil.TempFile(t, "temp_config")
	_, err = file.WriteString("test config")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)

	err = w.Add(file.Name())
	require.NoError(t, err)
	w.Start()
	file.Chmod(0777)
	require.NoError(t, err)
	require.Error(t, assertEvent(file.Name(), watcherCh), "timedout waiting for event")
}

func TestEventWatcherRemoveCreate(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		watcherCh <- event
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	require.NoError(t, err)
	file := testutil.TempFile(t, "temp_config")
	_, err = file.WriteString("test config")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)

	err = w.Add(file.Name())
	require.NoError(t, err)
	w.reconcileTimeout = 20 * time.Millisecond
	w.Start()
	err = os.Remove(file.Name())
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)
	recreated, err := os.Create(file.Name())
	require.NoError(t, err)
	require.NoError(t, assertEvent(file.Name(), watcherCh))
	_, err = recreated.WriteString("config 2")
	require.NoError(t, err)
	err = recreated.Sync()
	require.NoError(t, err)
	require.NoError(t, assertEvent(file.Name(), watcherCh))
}

func TestEventWatcherMove(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		watcherCh <- event
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	require.NoError(t, err)
	file := testutil.TempFile(t, "temp_config")
	_, err = file.WriteString("test config")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)

	require.NoError(t, err)
	file2 := testutil.TempFile(t, "temp_config"+randomString(12))
	_, err = file2.WriteString("test config 2")
	require.NoError(t, err)
	err = file2.Sync()
	require.NoError(t, err)
	err = file2.Close()
	require.NoError(t, err)

	err = w.Add(file2.Name())

	require.NoError(t, err)
	w.reconcileTimeout = 20 * time.Millisecond
	w.Start()
	os.Rename(file.Name(), file2.Name())
	require.NoError(t, assertEvent(file2.Name(), watcherCh))
}

func TestEventReconcile(t *testing.T) {
	watcherCh := make(chan *WatcherEvent)
	w, err := New(func(event *WatcherEvent) error {
		watcherCh <- event
		return nil
	})
	defer func() {
		_ = w.Close()
	}()
	require.NoError(t, err)
	file := testutil.TempFile(t, "temp_config")
	_, err = file.WriteString("test config")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)

	err = w.Add(file.Name())
	require.NoError(t, err)
	w.reconcileTimeout = 50 * time.Millisecond
	// remove the file from the internal watcher to only trigger the reconcile
	w.watcher.Remove(file.Name())
	w.Start()
	_, err = file.WriteString("test config 2")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)
	require.NoError(t, assertEvent(file.Name(), watcherCh))
}

func assertEvent(name string, watcherCh chan *WatcherEvent) error {
	timeout := time.After(200 * time.Millisecond)
	select {
	case ev := <-watcherCh:
		if ev.Filename != name {
			return fmt.Errorf("filename do not match")
		}
		return nil
	case <-timeout:
		return fmt.Errorf("timedout waiting for event")
	}
}
