package api

import (
	"bytes"
	"testing"
)

func TestSnapshot(t *testing.T) {
	t.Parallel()
	c, s := makeClient(t)
	defer s.Stop()

	// Place an initial key into the store.
	kv := c.KV()
	key := &KVPair{Key: testKey(), Value: []byte("hello")}
	if _, err := kv.Put(key, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Make sure it reads back.
	pair, _, err := kv.Get(key.Key, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if pair == nil {
		t.Fatalf("expected value: %#v", pair)
	}
	if !bytes.Equal(pair.Value, []byte("hello")) {
		t.Fatalf("unexpected value: %#v", pair)
	}

	// Take a snapshot.
	snapshot := c.Snapshot()
	snap, err := snapshot.Save(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer snap.Close()

	// Overwrite the key's value.
	key.Value = []byte("goodbye")
	if _, err := kv.Put(key, nil); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Read the key back and look for the new value.
	pair, _, err = kv.Get(key.Key, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if pair == nil {
		t.Fatalf("expected value: %#v", pair)
	}
	if !bytes.Equal(pair.Value, []byte("goodbye")) {
		t.Fatalf("unexpected value: %#v", pair)
	}

	// Restore the snapshot.
	if err := snapshot.Restore(nil, snap); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Read the key back and look for the original value.
	pair, _, err = kv.Get(key.Key, nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if pair == nil {
		t.Fatalf("expected value: %#v", pair)
	}
	if !bytes.Equal(pair.Value, []byte("hello")) {
		t.Fatalf("unexpected value: %#v", pair)
	}
}
