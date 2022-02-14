package config

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

func TestNewWatcher(t *testing.T) {
	w, err := NewFileWatcher(func(event *WatcherEvent) {}, []string{}, hclog.New(&hclog.LoggerOptions{}))
	require.NoError(t, err)
	require.NotNil(t, w)
}

func TestWatcherRenameEvent(t *testing.T) {

	fileTmp := createTempConfigFile(t, "temp_config3")
	filepaths := []string{createTempConfigFile(t, "temp_config1"), createTempConfigFile(t, "temp_config2")}
	watcherCh := make(chan *WatcherEvent)
	w, err := NewFileWatcher(func(event *WatcherEvent) {
		watcherCh <- event
	}, filepaths, hclog.New(&hclog.LoggerOptions{}))
	require.NoError(t, err)
	w.Start(context.Background())
	defer func() {
		_ = w.Close()
	}()

	require.NoError(t, err)
	err = os.Rename(fileTmp, filepaths[0])
	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	require.NoError(t, err)
	require.NoError(t, assertEvent(filepaths[0], watcherCh))
	// make sure we consume all events
	assertEvent(filepaths[0], watcherCh)
}

func TestWatcherAddNotExist(t *testing.T) {

	file := testutil.TempFile(t, "temp_config")
	filename := file.Name() + randomStr(16)
	w, err := NewFileWatcher(func(event *WatcherEvent) {
	}, []string{filename}, hclog.New(&hclog.LoggerOptions{}))
	require.Error(t, err, "no such file or directory")
	require.Nil(t, w)
}

func TestEventWatcherWrite(t *testing.T) {

	file := testutil.TempFile(t, "temp_config")
	_, err := file.WriteString("test config")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)
	watcherCh := make(chan *WatcherEvent)
	w, err := NewFileWatcher(func(event *WatcherEvent) {
		watcherCh <- event
	}, []string{file.Name()}, hclog.New(&hclog.LoggerOptions{}))
	require.NoError(t, err)
	w.Start(context.Background())
	defer func() {
		_ = w.Close()
	}()

	_, err = file.WriteString("test config 2")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)
	require.NoError(t, assertEvent(file.Name(), watcherCh))
}

func TestEventWatcherRead(t *testing.T) {

	filepath := createTempConfigFile(t, "temp_config1")
	watcherCh := make(chan *WatcherEvent)
	w, err := NewFileWatcher(func(event *WatcherEvent) {
		watcherCh <- event
	}, []string{filepath}, hclog.New(&hclog.LoggerOptions{}))
	require.NoError(t, err)
	w.Start(context.Background())
	defer func() {
		_ = w.Close()
	}()

	_, err = os.ReadFile(filepath)
	require.NoError(t, err)
	require.Error(t, assertEvent(filepath, watcherCh), "timedout waiting for event")
}

func TestEventWatcherChmod(t *testing.T) {
	file := testutil.TempFile(t, "temp_config")
	defer func() {
		err := file.Close()
		require.NoError(t, err)
	}()
	_, err := file.WriteString("test config")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)

	watcherCh := make(chan *WatcherEvent)
	w, err := NewFileWatcher(func(event *WatcherEvent) {
		watcherCh <- event
	}, []string{file.Name()}, hclog.New(&hclog.LoggerOptions{}))
	require.NoError(t, err)
	w.Start(context.Background())
	defer func() {
		_ = w.Close()
	}()

	file.Chmod(0777)
	require.NoError(t, err)
	require.Error(t, assertEvent(file.Name(), watcherCh), "timedout waiting for event")
}

func TestEventWatcherRemoveCreate(t *testing.T) {

	filepath := createTempConfigFile(t, "temp_config1")
	watcherCh := make(chan *WatcherEvent)
	w, err := NewFileWatcher(func(event *WatcherEvent) {
		watcherCh <- event
	}, []string{filepath}, hclog.New(&hclog.LoggerOptions{}))
	require.NoError(t, err)
	w.Start(context.Background())
	defer func() {
		_ = w.Close()
	}()

	require.NoError(t, err)
	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	err = os.Remove(filepath)
	require.NoError(t, err)
	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	recreated, err := os.Create(filepath)
	require.NoError(t, err)
	_, err = recreated.WriteString("config 2")
	require.NoError(t, err)
	err = recreated.Sync()
	require.NoError(t, err)
	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	// this an event coming from the reconcile loop
	require.NoError(t, assertEvent(filepath, watcherCh))
}

func TestEventWatcherMove(t *testing.T) {

	filepath := createTempConfigFile(t, "temp_config1")
	watcherCh := make(chan *WatcherEvent)
	w, err := NewFileWatcher(func(event *WatcherEvent) {
		watcherCh <- event
	}, []string{filepath}, hclog.New(&hclog.LoggerOptions{}))
	require.NoError(t, err)
	w.Start(context.Background())
	defer func() {
		_ = w.Close()
	}()

	for i := 0; i < 10; i++ {
		filepath2 := createTempConfigFile(t, "temp_config2")
		err = os.Rename(filepath2, filepath)
		require.NoError(t, err)
		require.NoError(t, assertEvent(filepath, watcherCh))
	}
}

func TestEventReconcileMove(t *testing.T) {
	filepath := createTempConfigFile(t, "temp_config1")
	filepath2 := createTempConfigFile(t, "temp_config2")
	watcherCh := make(chan *WatcherEvent)
	w, err := NewFileWatcher(func(event *WatcherEvent) {
		watcherCh <- event
	}, []string{filepath}, hclog.New(&hclog.LoggerOptions{}))
	require.NoError(t, err)
	w.Start(context.Background())
	defer func() {
		_ = w.Close()
	}()

	// remove the file from the internal watcher to only trigger the reconcile
	err = w.watcher.Remove(filepath)
	require.NoError(t, err)

	err = os.Rename(filepath2, filepath)
	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	require.NoError(t, err)
	require.NoError(t, assertEvent(filepath, watcherCh))
}

func TestEventWatcherDirCreateRemove(t *testing.T) {
	filepath := createTempConfigDir(t, "temp_config1")
	watcherCh := make(chan *WatcherEvent)
	w, err := NewFileWatcher(func(event *WatcherEvent) {
		watcherCh <- event
	}, []string{filepath}, hclog.New(&hclog.LoggerOptions{}))
	require.NoError(t, err)
	w.Start(context.Background())
	defer func() {
		_ = w.Close()
	}()
	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	for i := 0; i < 10; i++ {
		name := filepath + "/" + randomStr(20)
		file, err := os.Create(name)
		require.NoError(t, err)
		err = file.Close()
		require.NoError(t, err)
		require.NoError(t, assertEvent(filepath, watcherCh))

		err = os.Remove(name)
		require.NoError(t, err)
		require.NoError(t, assertEvent(filepath, watcherCh))
	}
}

func TestEventWatcherDirMove(t *testing.T) {
	filepath := createTempConfigDir(t, "temp_config1")

	name := filepath + "/" + randomStr(20)
	file, err := os.Create(name)
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)
	watcherCh := make(chan *WatcherEvent)
	w, err := NewFileWatcher(func(event *WatcherEvent) {
		watcherCh <- event
	}, []string{filepath}, hclog.New(&hclog.LoggerOptions{}))
	require.NoError(t, err)
	w.Start(context.Background())
	defer func() {
		_ = w.Close()
	}()

	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	for i := 0; i < 100; i++ {
		filepathTmp := createTempConfigFile(t, "temp_config2")
		os.Rename(filepathTmp, name)
		require.NoError(t, err)
		require.NoError(t, assertEvent(filepath, watcherCh))
	}
}

func TestEventWatcherDirMoveTrim(t *testing.T) {
	filepath := createTempConfigDir(t, "temp_config1")

	name := filepath + "/" + randomStr(20)
	file, err := os.Create(name)
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)
	watcherCh := make(chan *WatcherEvent)
	w, err := NewFileWatcher(func(event *WatcherEvent) {
		watcherCh <- event
	}, []string{filepath + "/"}, hclog.New(&hclog.LoggerOptions{}))
	require.NoError(t, err)
	w.Start(context.Background())
	defer func() {
		_ = w.Close()
	}()

	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	for i := 0; i < 100; i++ {
		filepathTmp := createTempConfigFile(t, "temp_config2")
		os.Rename(filepathTmp, name)
		require.NoError(t, err)
		require.NoError(t, assertEvent(filepath, watcherCh))
	}
}

// Consul do not support configuration in sub-directories
func TestEventWatcherSubDirMove(t *testing.T) {
	filepath := createTempConfigDir(t, "temp_config1")
	err := os.Mkdir(filepath+"/temp", 0777)
	require.NoError(t, err)
	name := filepath + "/temp/" + randomStr(20)
	file, err := os.Create(name)
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)
	watcherCh := make(chan *WatcherEvent)
	w, err := NewFileWatcher(func(event *WatcherEvent) {
		watcherCh <- event
	}, []string{filepath}, hclog.New(&hclog.LoggerOptions{}))
	require.NoError(t, err)
	w.Start(context.Background())
	defer func() {
		_ = w.Close()
	}()

	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	for i := 0; i < 2; i++ {
		filepathTmp := createTempConfigFile(t, "temp_config2")
		os.Rename(filepathTmp, name)
		require.NoError(t, err)
		require.Error(t, assertEvent(filepath, watcherCh), "timedout waiting for event")
	}
}

func TestEventWatcherDirRead(t *testing.T) {
	filepath := createTempConfigDir(t, "temp_config1")

	name := filepath + "/" + randomStr(20)
	file, err := os.Create(name)
	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)
	watcherCh := make(chan *WatcherEvent)
	w, err := NewFileWatcher(func(event *WatcherEvent) {
		watcherCh <- event
	}, []string{filepath}, hclog.New(&hclog.LoggerOptions{}))
	require.NoError(t, err)
	w.Start(context.Background())
	defer func() {
		_ = w.Close()
	}()

	time.Sleep(w.reconcileTimeout + 50*time.Millisecond)
	_, err = os.ReadFile(name)
	require.NoError(t, err)
	require.Error(t, assertEvent(filepath, watcherCh), "timedout waiting for event")
}

func TestEventWatcherMoveSoftLink(t *testing.T) {

	filepath := createTempConfigFile(t, "temp_config1")
	tempDir := createTempConfigDir(t, "temp_dir")
	name := tempDir + "/" + randomStr(20)
	err := os.Symlink(filepath, name)
	require.NoError(t, err)

	watcherCh := make(chan *WatcherEvent)
	w, err := NewFileWatcher(func(event *WatcherEvent) {
		watcherCh <- event
	}, []string{name}, hclog.New(&hclog.LoggerOptions{}))
	require.Error(t, err, "symbolic link are not supported")
	require.Nil(t, w)

}

func assertEvent(name string, watcherCh chan *WatcherEvent) error {
	timeout := time.After(500 * time.Millisecond)
	select {
	case ev := <-watcherCh:
		if ev.Filename != name && !strings.Contains(ev.Filename, name) {
			return fmt.Errorf("filename do not match %s %s", ev.Filename, name)
		}
		return nil
	case <-timeout:
		return fmt.Errorf("timedout waiting for event")
	}
}

func createTempConfigFile(t *testing.T, filename string) string {
	file := testutil.TempFile(t, filename)
	defer func() {
		err := file.Close()
		require.NoError(t, err)
	}()
	_, err := file.WriteString("test config")
	require.NoError(t, err)
	err = file.Sync()
	require.NoError(t, err)
	return file.Name()
}

func createTempConfigDir(t *testing.T, dirname string) string {
	return testutil.TempDir(t, dirname)
}
func randomStr(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz" +
		"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var seededRand *rand.Rand = rand.New(
		rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
