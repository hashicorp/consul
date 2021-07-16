package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	osexec "os/exec"
	"path"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/consul/agent/exec"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/go-hclog"
)

const (
	// remoteExecFileName is the name of the file we append to
	// the path, e.g. _rexec/session_id/job
	remoteExecFileName = "job"

	// rExecAck is the suffix added to an ack path
	remoteExecAckSuffix = "ack"

	// remoteExecAck is the suffix added to an exit code
	remoteExecExitSuffix = "exit"

	// remoteExecOutputDivider is used to namespace the output
	remoteExecOutputDivider = "out"

	// remoteExecOutputSize is the size we chunk output too
	remoteExecOutputSize = 4 * 1024

	// remoteExecOutputDeadline is how long we wait before uploading
	// less than the chunk size
	remoteExecOutputDeadline = 500 * time.Millisecond
)

// remoteExecSpec is used as the specification of the remote exec.
// It is stored in the KV store
type remoteExecSpec struct {
	Command string
	Args    []string
	Script  []byte
	Wait    time.Duration
}

type rexecWriter struct {
	BufCh    chan []byte
	BufSize  int
	BufIdle  time.Duration
	CancelCh chan struct{}

	buf     []byte
	bufLen  int
	bufLock sync.Mutex
	flush   *time.Timer
}

func (r *rexecWriter) Write(b []byte) (int, error) {
	r.bufLock.Lock()
	defer r.bufLock.Unlock()
	if r.flush != nil {
		r.flush.Stop()
		r.flush = nil
	}
	inpLen := len(b)
	if r.buf == nil {
		r.buf = make([]byte, r.BufSize)
	}

COPY:
	remain := len(r.buf) - r.bufLen
	if remain > len(b) {
		copy(r.buf[r.bufLen:], b)
		r.bufLen += len(b)
	} else {
		copy(r.buf[r.bufLen:], b[:remain])
		b = b[remain:]
		r.bufLen += remain
		r.bufLock.Unlock()
		r.Flush()
		r.bufLock.Lock()
		goto COPY
	}

	r.flush = time.AfterFunc(r.BufIdle, r.Flush)
	return inpLen, nil
}

func (r *rexecWriter) Flush() {
	r.bufLock.Lock()
	defer r.bufLock.Unlock()
	if r.flush != nil {
		r.flush.Stop()
		r.flush = nil
	}
	if r.bufLen == 0 {
		return
	}
	select {
	case r.BufCh <- r.buf[:r.bufLen]:
		r.buf = make([]byte, r.BufSize)
		r.bufLen = 0
	case <-r.CancelCh:
		r.bufLen = 0
	}
}

type remoteExecHandler struct {
	Logger       hclog.Logger
	AgentTokener AgentTokener
	KV           KV
}

type AgentTokener interface {
	AgentToken() string
}

type KV interface {
	Get(ctx context.Context, req structs.KeyRequest) (structs.IndexedDirEntries, error)
	Apply(ctx context.Context, req structs.KVSRequest) (bool, error)
}

type remoteExecEvent struct {
	NodeName   string
	Datacenter string
	Prefix     string
	Session    string
}

// newRemoteExecEventFromUserEvent creates and returns a remoteExecEvent by
// decoding the msg.Payload and adding NodeName and Datacenter to the event.
func newRemoteExecEventFromUserEvent(msg UserEvent, nn string, dc string) (remoteExecEvent, error) {
	var event remoteExecEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return event, fmt.Errorf("failed to decode remote exec event: %w", err)
	}
	event.NodeName = nn
	event.Datacenter = dc
	return event, nil
}

// handle is invoked when a new remote exec request is received
func (e *remoteExecHandler) handle(event remoteExecEvent) {

	var spec remoteExecSpec
	if !e.getExecSpec(event, &spec) {
		return
	}

	// Write the acknowledgement
	if !e.writeAck(event) {
		return
	}

	// Ensure we write out an exit code
	exitCode := 0
	defer e.writeExitCode(event, &exitCode)

	// Check if this is a script, we may need to spill to disk
	var script string
	if len(spec.Script) != 0 {
		tmpFile, err := ioutil.TempFile("", "rexec")
		if err != nil {
			e.Logger.Debug("failed to make tmp file", "error", err)
			exitCode = 255
			return
		}
		defer os.Remove(tmpFile.Name())
		os.Chmod(tmpFile.Name(), 0750)
		tmpFile.Write(spec.Script)
		tmpFile.Close()
		script = tmpFile.Name()
	} else {
		script = spec.Command
	}

	// Create the exec.Cmd
	e.Logger.Info("remote exec script", "script", script)
	var cmd *osexec.Cmd
	var err error
	if len(spec.Args) > 0 {
		cmd, err = exec.Subprocess(spec.Args)
	} else {
		cmd, err = exec.Script(script)
	}
	if err != nil {
		e.Logger.Debug("failed to start remote exec", "error", err)
		exitCode = 255
		return
	}

	// Setup the output streaming
	writer := &rexecWriter{
		BufCh:    make(chan []byte, 16),
		BufSize:  remoteExecOutputSize,
		BufIdle:  remoteExecOutputDeadline,
		CancelCh: make(chan struct{}),
	}
	cmd.Stdout = writer
	cmd.Stderr = writer

	// Start execution
	if err := cmd.Start(); err != nil {
		e.Logger.Debug("failed to start remote exec", "error", err)
		exitCode = 255
		return
	}

	// Wait for the process to exit
	exitCh := make(chan int, 1)
	go func() {
		err := cmd.Wait()
		writer.Flush()
		close(writer.BufCh)
		if err == nil {
			exitCh <- 0
			return
		}

		// Try to determine the exit code
		if exitErr, ok := err.(*osexec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCh <- status.ExitStatus()
				return
			}
		}
		exitCh <- 1
	}()

	// Wait until we are complete, uploading as we go
WAIT:
	for num := 0; ; num++ {
		select {
		case out := <-writer.BufCh:
			if out == nil {
				break WAIT
			}
			if !e.writeOutput(event, num, out) {
				close(writer.CancelCh)
				exitCode = 255
				return
			}
		case <-time.After(spec.Wait):
			// Acts like a heartbeat, since there is no output
			if !e.writeOutput(event, num, nil) {
				close(writer.CancelCh)
				exitCode = 255
				return
			}
		}
	}

	// Get the exit code
	exitCode = <-exitCh
}

// getExecSpec is used to get the exec specification.
// Returns if execution should continue
// TODO: return error instead of bool and log in the caller.
func (e *remoteExecHandler) getExecSpec(event remoteExecEvent, spec *remoteExecSpec) bool {
	get := structs.KeyRequest{
		Datacenter: event.Datacenter,
		Key:        path.Join(event.Prefix, event.Session, remoteExecFileName),
		QueryOptions: structs.QueryOptions{
			AllowStale: true, // Stale read for scale! Retry on failure.
		},
	}
	get.Token = e.AgentTokener.AgentToken()
QUERY:
	out, err := e.KV.Get(context.TODO(), get)
	if err != nil {
		e.Logger.Error("failed to get remote exec job", "error", err)
		return false
	}
	if len(out.Entries) == 0 {
		// If the initial read was stale and had no data, retry as a consistent read
		if get.QueryOptions.AllowStale {
			e.Logger.Debug("trying consistent fetch of remote exec job spec")
			get.QueryOptions.AllowStale = false
			goto QUERY
		} else {
			e.Logger.Debug("remote exec aborted, job spec missing")
			return false
		}
	}
	if err := json.Unmarshal(out.Entries[0].Value, &spec); err != nil {
		e.Logger.Error("failed to decode remote exec spec", "error", err)
		return false
	}
	return true
}

// writeAck is used to write an ack. Returns if execution should
// continue.
func (e *remoteExecHandler) writeAck(event remoteExecEvent) bool {
	if err := e.writeKey(event, remoteExecAckSuffix, nil); err != nil {
		e.Logger.Error("failed to ack remote exec job", "error", err)
		return false
	}
	return true
}

// writeOutput is used to write output
func (e *remoteExecHandler) writeOutput(event remoteExecEvent, num int, output []byte) bool {
	suffix := path.Join(remoteExecOutputDivider, fmt.Sprintf("%05x", num))
	if err := e.writeKey(event, suffix, output); err != nil {
		e.Logger.Error("failed to write output for remote exec job", "error", err)
		return false
	}
	return true
}

// writeExitCode is used to write an exit code
func (e *remoteExecHandler) writeExitCode(event remoteExecEvent, exitCode *int) bool {
	val := []byte(strconv.FormatInt(int64(*exitCode), 10))
	if err := e.writeKey(event, remoteExecExitSuffix, val); err != nil {
		e.Logger.Error("failed to write exit code for remote exec job", "error", err)
		return false
	}
	return true
}

// writeKey is used to write an output key for a remote exec job
func (e *remoteExecHandler) writeKey(event remoteExecEvent, suffix string, val []byte) error {
	key := path.Join(event.Prefix, event.Session, event.NodeName, suffix)
	write := structs.KVSRequest{
		Datacenter: event.Datacenter,
		Op:         api.KVLock,
		DirEnt: structs.DirEntry{
			Key:     key,
			Value:   val,
			Session: event.Session,
		},
	}
	write.WriteRequest.Token = e.AgentTokener.AgentToken()
	success, err := e.KV.Apply(context.TODO(), write)
	if err != nil {
		return err
	}
	if !success {
		return fmt.Errorf("write failed")
	}
	return nil
}
