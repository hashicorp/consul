package state

import (
	"reflect"
	"testing"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
	"github.com/pascaldekloe/goe/verify"
)

func TestStateStore_ACLBootstrap(t *testing.T) {
	acl1 := &structs.ACL{
		ID:   "03f43a07-7e78-1f72-6c72-5a4e3b1ac3df",
		Type: structs.ACLTypeManagement,
	}

	acl2 := &structs.ACL{
		ID:   "0546a993-aa7a-741e-fb7f-09159ae56ec1",
		Type: structs.ACLTypeManagement,
	}

	setup := func() *Store {
		s := testStateStore(t)

		// The clean state store should initially have no bootstrap record.
		bs, err := s.ACLGetBootstrap()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if bs != nil {
			t.Fatalf("bad: %#v", bs)
		}

		// Make sure that a bootstrap attempt fails in this state.
		if err := s.ACLBootstrap(1, acl1); err != structs.ACLBootstrapNotInitializedErr {
			t.Fatalf("err: %v", err)
		}
		_, gotA, err := s.ACLList(nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		verify.Values(t, "", gotA, structs.ACLs{})

		// Initialize bootstrapping.
		enabled, err := s.ACLBootstrapInit(2)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if !enabled {
			t.Fatalf("bad")
		}

		// Read it back.
		gotB, err := s.ACLGetBootstrap()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		wantB := &structs.ACLBootstrap{
			AllowBootstrap: true,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 2,
				ModifyIndex: 2,
			},
		}
		verify.Values(t, "", gotB, wantB)

		return s
	}

	// This is the bootstrap happy path.
	t.Run("bootstrap", func(t *testing.T) {
		s := setup()

		// Perform a regular bootstrap.
		if err := s.ACLBootstrap(3, acl1); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Read it back.
		gotB, err := s.ACLGetBootstrap()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		wantB := &structs.ACLBootstrap{
			AllowBootstrap: false,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 2,
				ModifyIndex: 3,
			},
		}
		verify.Values(t, "", gotB, wantB)

		// Make sure another attempt fails.
		if err := s.ACLBootstrap(4, acl2); err != structs.ACLBootstrapNotAllowedErr {
			t.Fatalf("err: %v", err)
		}

		// Check that the bootstrap state remains the same.
		gotB, err = s.ACLGetBootstrap()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		verify.Values(t, "", gotB, wantB)

		// Make sure the ACLs are in an expected state.
		_, gotA, err := s.ACLList(nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		wantA := structs.ACLs{
			&structs.ACL{
				ID:   acl1.ID,
				Type: acl1.Type,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 3,
					ModifyIndex: 3,
				},
			},
		}
		verify.Values(t, "", gotA, wantA)
	})

	// This case initialized bootstrap but it gets canceled because a
	// management token gets created manually.
	t.Run("bootstrap canceled", func(t *testing.T) {
		s := setup()

		// Make a management token manually.
		if err := s.ACLSet(3, acl1); err != nil {
			t.Fatalf("err: %v", err)
		}

		// Bootstrapping should have gotten disabled.
		gotB, err := s.ACLGetBootstrap()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		wantB := &structs.ACLBootstrap{
			AllowBootstrap: false,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 2,
				ModifyIndex: 3,
			},
		}
		verify.Values(t, "", gotB, wantB)

		// Make sure another attempt fails.
		if err := s.ACLBootstrap(4, acl2); err != structs.ACLBootstrapNotAllowedErr {
			t.Fatalf("err: %v", err)
		}

		// Check that the bootstrap state remains the same.
		gotB, err = s.ACLGetBootstrap()
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		verify.Values(t, "", gotB, wantB)

		// Make sure the ACLs are in an expected state.
		_, gotA, err := s.ACLList(nil)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		wantA := structs.ACLs{
			&structs.ACL{
				ID:   acl1.ID,
				Type: acl1.Type,
				RaftIndex: structs.RaftIndex{
					CreateIndex: 3,
					ModifyIndex: 3,
				},
			},
		}
		verify.Values(t, "", gotA, wantA)
	})
}

func TestStateStore_ACLBootstrap_InitialTokens(t *testing.T) {
	acl1 := &structs.ACL{
		ID:   "03f43a07-7e78-1f72-6c72-5a4e3b1ac3df",
		Type: structs.ACLTypeManagement,
	}

	acl2 := &structs.ACL{
		ID:   "0546a993-aa7a-741e-fb7f-09159ae56ec1",
		Type: structs.ACLTypeManagement,
	}

	s := testStateStore(t)

	// Make a management token manually. This also makes sure that it's ok
	// to set a token if bootstrap has not been initialized.
	if err := s.ACLSet(1, acl1); err != nil {
		t.Fatalf("err: %v", err)
	}

	// Initialize bootstrapping, which should not be enabled since an
	// existing token is present.
	enabled, err := s.ACLBootstrapInit(2)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if enabled {
		t.Fatalf("bad")
	}

	// Read it back.
	gotB, err := s.ACLGetBootstrap()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	wantB := &structs.ACLBootstrap{
		AllowBootstrap: false,
		RaftIndex: structs.RaftIndex{
			CreateIndex: 2,
			ModifyIndex: 2,
		},
	}
	verify.Values(t, "", gotB, wantB)

	// Make sure an attempt fails.
	if err := s.ACLBootstrap(3, acl2); err != structs.ACLBootstrapNotAllowedErr {
		t.Fatalf("err: %v", err)
	}

	// Check that the bootstrap state remains the same.
	gotB, err = s.ACLGetBootstrap()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	verify.Values(t, "", gotB, wantB)

	// Make sure the ACLs are in an expected state.
	_, gotA, err := s.ACLList(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	wantA := structs.ACLs{
		&structs.ACL{
			ID:   acl1.ID,
			Type: acl1.Type,
			RaftIndex: structs.RaftIndex{
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
	}
	verify.Values(t, "", gotA, wantA)
}

func TestStateStore_ACLBootstrap_Snapshot_Restore(t *testing.T) {
	s := testStateStore(t)

	enabled, err := s.ACLBootstrapInit(1)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !enabled {
		t.Fatalf("bad")
	}

	gotB, err := s.ACLGetBootstrap()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	wantB := &structs.ACLBootstrap{
		AllowBootstrap: true,
		RaftIndex: structs.RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 1,
		},
	}
	verify.Values(t, "", gotB, wantB)

	snap := s.Snapshot()
	defer snap.Close()
	bs, err := snap.ACLBootstrap()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	verify.Values(t, "", bs, wantB)

	r := testStateStore(t)
	restore := r.Restore()
	if err := restore.ACLBootstrap(bs); err != nil {
		t.Fatalf("err: %v", err)
	}
	restore.Commit()

	gotB, err = r.ACLGetBootstrap()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	verify.Values(t, "", gotB, wantB)
}

func TestStateStore_ACLSet_ACLGet(t *testing.T) {
	s := testStateStore(t)

	// Querying ACLs with no results returns nil
	ws := memdb.NewWatchSet()
	idx, res, err := s.ACLGet(ws, "nope")
	if idx != 0 || res != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}

	// Inserting an ACL with empty ID is disallowed
	if err := s.ACLSet(1, &structs.ACL{}); err == nil {
		t.Fatalf("expected %#v, got: %#v", ErrMissingACLID, err)
	}
	if watchFired(ws) {
		t.Fatalf("bad")
	}

	// Index is not updated if nothing is saved
	if idx := s.maxIndex("acls"); idx != 0 {
		t.Fatalf("bad index: %d", idx)
	}

	// Inserting valid ACL works
	acl := &structs.ACL{
		ID:    "acl1",
		Name:  "First ACL",
		Type:  structs.ACLTypeClient,
		Rules: "rules1",
	}
	if err := s.ACLSet(1, acl); err != nil {
		t.Fatalf("err: %s", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Check that the index was updated
	if idx := s.maxIndex("acls"); idx != 1 {
		t.Fatalf("bad index: %d", idx)
	}

	// Retrieve the ACL again
	ws = memdb.NewWatchSet()
	idx, result, err := s.ACLGet(ws, "acl1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 1 {
		t.Fatalf("bad index: %d", idx)
	}

	// Check that the ACL matches the result
	expect := &structs.ACL{
		ID:    "acl1",
		Name:  "First ACL",
		Type:  structs.ACLTypeClient,
		Rules: "rules1",
		RaftIndex: structs.RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 1,
		},
	}
	if !reflect.DeepEqual(result, expect) {
		t.Fatalf("bad: %#v", result)
	}

	// Update the ACL
	acl = &structs.ACL{
		ID:    "acl1",
		Name:  "First ACL",
		Type:  structs.ACLTypeClient,
		Rules: "rules2",
	}
	if err := s.ACLSet(2, acl); err != nil {
		t.Fatalf("err: %s", err)
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Index was updated
	if idx := s.maxIndex("acls"); idx != 2 {
		t.Fatalf("bad: %d", idx)
	}

	// ACL was updated and matches expected value
	expect = &structs.ACL{
		ID:    "acl1",
		Name:  "First ACL",
		Type:  structs.ACLTypeClient,
		Rules: "rules2",
		RaftIndex: structs.RaftIndex{
			CreateIndex: 1,
			ModifyIndex: 2,
		},
	}
	if !reflect.DeepEqual(acl, expect) {
		t.Fatalf("bad: %#v", acl)
	}
}

func TestStateStore_ACLList(t *testing.T) {
	s := testStateStore(t)

	// Listing when no ACLs exist returns nil
	ws := memdb.NewWatchSet()
	idx, res, err := s.ACLList(ws)
	if idx != 0 || res != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}

	// Insert some ACLs
	acls := structs.ACLs{
		&structs.ACL{
			ID:    "acl1",
			Type:  structs.ACLTypeClient,
			Rules: "rules1",
			RaftIndex: structs.RaftIndex{
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
		&structs.ACL{
			ID:    "acl2",
			Type:  structs.ACLTypeClient,
			Rules: "rules2",
			RaftIndex: structs.RaftIndex{
				CreateIndex: 2,
				ModifyIndex: 2,
			},
		},
	}
	for _, acl := range acls {
		if err := s.ACLSet(acl.ModifyIndex, acl); err != nil {
			t.Fatalf("err: %s", err)
		}
	}
	if !watchFired(ws) {
		t.Fatalf("bad")
	}

	// Query the ACLs
	idx, res, err = s.ACLList(nil)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}

	// Check that the result matches
	if !reflect.DeepEqual(res, acls) {
		t.Fatalf("bad: %#v", res)
	}
}

func TestStateStore_ACLDelete(t *testing.T) {
	s := testStateStore(t)

	// Calling delete on an ACL which doesn't exist returns nil
	if err := s.ACLDelete(1, "nope"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Index isn't updated if nothing is deleted
	if idx := s.maxIndex("acls"); idx != 0 {
		t.Fatalf("bad index: %d", idx)
	}

	// Insert an ACL
	if err := s.ACLSet(1, &structs.ACL{ID: "acl1"}); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Delete the ACL and check that the index was updated
	if err := s.ACLDelete(2, "acl1"); err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx := s.maxIndex("acls"); idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}

	tx := s.db.Txn(false)
	defer tx.Abort()

	// Check that the ACL was really deleted
	result, err := tx.First("acls", "id", "acl1")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if result != nil {
		t.Fatalf("expected nil, got: %#v", result)
	}
}

func TestStateStore_ACL_Snapshot_Restore(t *testing.T) {
	s := testStateStore(t)

	// Insert some ACLs.
	acls := structs.ACLs{
		&structs.ACL{
			ID:    "acl1",
			Type:  structs.ACLTypeClient,
			Rules: "rules1",
			RaftIndex: structs.RaftIndex{
				CreateIndex: 1,
				ModifyIndex: 1,
			},
		},
		&structs.ACL{
			ID:    "acl2",
			Type:  structs.ACLTypeClient,
			Rules: "rules2",
			RaftIndex: structs.RaftIndex{
				CreateIndex: 2,
				ModifyIndex: 2,
			},
		},
	}
	for _, acl := range acls {
		if err := s.ACLSet(acl.ModifyIndex, acl); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Snapshot the ACLs.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	if err := s.ACLDelete(3, "acl1"); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Verify the snapshot.
	if idx := snap.LastIndex(); idx != 2 {
		t.Fatalf("bad index: %d", idx)
	}
	iter, err := snap.ACLs()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	var dump structs.ACLs
	for acl := iter.Next(); acl != nil; acl = iter.Next() {
		dump = append(dump, acl.(*structs.ACL))
	}
	if !reflect.DeepEqual(dump, acls) {
		t.Fatalf("bad: %#v", dump)
	}

	// Restore the values into a new state store.
	func() {
		s := testStateStore(t)
		restore := s.Restore()
		for _, acl := range dump {
			if err := restore.ACL(acl); err != nil {
				t.Fatalf("err: %s", err)
			}
		}
		restore.Commit()

		// Read the restored ACLs back out and verify that they match.
		idx, res, err := s.ACLList(nil)
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		if idx != 2 {
			t.Fatalf("bad index: %d", idx)
		}
		if !reflect.DeepEqual(res, acls) {
			t.Fatalf("bad: %#v", res)
		}

		// Check that the index was updated.
		if idx := s.maxIndex("acls"); idx != 2 {
			t.Fatalf("bad index: %d", idx)
		}
	}()
}
