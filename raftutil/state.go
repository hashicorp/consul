// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package raftutil

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/consul-net-rpc/go-msgpack/codec"

	"github.com/hashicorp/consul/agent/structs"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
	"go.etcd.io/bbolt"
)

var (
	errAlreadyOpen = errors.New("unable to open raft logs that are in use")
)

// RaftStateInfo returns info about the nomad state, as found in the passed data-dir directory
func RaftStateInfo(p string) (store *raftboltdb.BoltStore, firstIdx uint64, lastIdx uint64, err error) {
	opts := raftboltdb.Options{
		Path: p,
		BoltOptions: &bbolt.Options{
			ReadOnly: true,
			Timeout:  1 * time.Second,
		},
	}
	s, err := raftboltdb.New(opts)
	if err != nil {
		if strings.HasSuffix(err.Error(), "timeout") {
			return nil, 0, 0, errAlreadyOpen
		}
		return nil, 0, 0, fmt.Errorf("failed to open raft logs: %v", err)
	}

	firstIdx, err = s.FirstIndex()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to fetch first index: %v", err)
	}

	lastIdx, err = s.LastIndex()
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to fetch last index: %v", err)
	}

	return s, firstIdx, lastIdx, nil
}

// LogEntries reads the raft logs found in the data directory found at
// the path `p`, and returns a channel of logs, and a channel of
// warnings. If opening the raft state returns an error, both channels
// will be nil.
func LogEntries(p string) (<-chan interface{}, <-chan error, error) {
	store, firstIdx, lastIdx, err := RaftStateInfo(p)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open raft logs: %v", err)
	}

	entries := make(chan interface{})
	warnings := make(chan error)

	go func() {
		defer store.Close()
		defer close(entries)
		for i := firstIdx; i <= lastIdx; i++ {
			var e raft.Log
			err := store.GetLog(i, &e)
			if err != nil {
				warnings <- fmt.Errorf(
					"failed to read log entry at index %d (firstIdx: %d, lastIdx: %d): %v",
					i, firstIdx, lastIdx, err)
				continue
			}

			entry, err := decode(&e)
			if err != nil {
				warnings <- fmt.Errorf(
					"failed to decode log entry at index %d: %v", i, err)
				continue
			}

			entries <- entry
		}
	}()

	return entries, warnings, nil
}

type logMessage struct {
	LogType string
	Term    uint64
	Index   uint64

	CommandType           string      `json:",omitempty"`
	IgnoreUnknownTypeFlag bool        `json:",omitempty"`
	Body                  interface{} `json:",omitempty"`
}

func decode(e *raft.Log) (*logMessage, error) {
	m := &logMessage{
		LogType: logTypes[e.Type],
		Term:    e.Term,
		Index:   e.Index,
	}

	if m.LogType == "" {
		m.LogType = fmt.Sprintf("%d", e.Type)
	}

	var data []byte
	if e.Type == raft.LogCommand {
		if len(e.Data) == 0 {
			return nil, fmt.Errorf("command did not include data")
		}

		msgType := structs.MessageType(e.Data[0])

		m.CommandType = commandName(msgType & ^structs.IgnoreUnknownTypeFlag)
		m.IgnoreUnknownTypeFlag = (msgType & structs.IgnoreUnknownTypeFlag) != 0

		data = e.Data[1:]
	} else {
		data = e.Data
	}

	if len(data) != 0 {
		decoder := codec.NewDecoder(bytes.NewReader(data), structs.MsgpackHandle)

		var v interface{}
		var err error

		var vr interface{}
		err = decoder.Decode(&vr)
		v = vr

		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to decode log entry at index %d: failed to decode body of %v.%v %v\n", e.Index, e.Type, m.CommandType, err)
			v = "FAILED TO DECODE DATA"
		}
		fixTime(v)
		m.Body = v
	}

	return m, nil
}

var logTypes = map[raft.LogType]string{
	raft.LogCommand:              "LogCommand",
	raft.LogNoop:                 "LogNoop",
	raft.LogAddPeerDeprecated:    "LogAddPeerDeprecated",
	raft.LogRemovePeerDeprecated: "LogRemovePeerDeprecated",
	raft.LogBarrier:              "LogBarrier",
	raft.LogConfiguration:        "LogConfiguration",
}

func commandName(mt structs.MessageType) string {
	n := msgTypeNames[mt]
	if n != "" {
		return n
	}

	return fmt.Sprintf("%v", mt)
}

// FindRaftFile finds raft.db and returns path
func FindRaftFile(p string) (raftpath string, err error) {
	// Try known locations before traversal to avoid walking deep structure
	if _, err = os.Stat(filepath.Join(p, "server", "raft", "raft.db")); err == nil {
		raftpath = filepath.Join(p, "server", "raft", "raft.db")
	} else if _, err = os.Stat(filepath.Join(p, "raft", "raft.db")); err == nil {
		raftpath = filepath.Join(p, "raft", "raft.db")
	} else if _, err = os.Stat(filepath.Join(p, "raft.db")); err == nil {
		raftpath = filepath.Join(p, "raft.db")
	} else if _, err = os.Stat(p); err == nil && filepath.Ext(p) == ".db" {
		// Also accept path to .db file
		raftpath = p
	} else {
		raftpath, err = FindFileInPath("raft.db", p)
	}

	if err != nil {
		return "", err
	}

	return raftpath, nil
}

// FindRaftDir finds raft.db and returns parent directory path
func FindRaftDir(p string) (raftpath string, err error) {
	raftpath, err = FindRaftFile(p)
	if err != nil {
		return "", err
	}

	if raftpath == "" {
		return "", fmt.Errorf("failed to find raft dir in %s", p)
	}

	return filepath.Dir(raftpath), nil
}

// FindFileInPath searches for file in path p
func FindFileInPath(file string, p string) (path string, err error) {
	// Define walk function to find file
	walkFn := func(walkPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if filepath.Base(walkPath) == file {
			path = walkPath
		}
		return nil
	}

	// Walk path p looking for file
	walkErr := filepath.Walk(p, walkFn)
	if walkErr != nil {
		return "", walkErr
	}

	if path == "" {
		return "", fmt.Errorf("File %s not found in path %s", file, p)
	}

	return path, nil
}
