package state

import (
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/consul/consul/structs"
)

func TestStateStore_Query_QuerySet_QueryGet(t *testing.T) {
	s := testStateStore(t)

	// Querying with no results returns nil.
	idx, res, err := s.QueryGet(testUUID())
	if idx != 0 || res != nil || err != nil {
		t.Fatalf("expected (0, nil, nil), got: (%d, %#v, %#v)", idx, res, err)
	}

	// Inserting a query with empty ID is disallowed.
	if err := s.QuerySet(1, &structs.PreparedQuery{}); err == nil {
		t.Fatalf("expected %#v, got: %#v", ErrMissingQueryID, err)
	}

	// Index is not updated if nothing is saved.
	if idx := s.maxIndex("queries"); idx != 0 {
		t.Fatalf("bad index: %d", idx)
	}

	// Build a legit-looking query with the most basic options.
	query := &structs.PreparedQuery{
		ID: testUUID(),
		Service: structs.ServiceQuery{
			Service: "redis",
		},
	}

	// The set will still fail because the service isn't registered yet.
	err = s.QuerySet(1, query)
	if err == nil || !strings.Contains(err.Error(), "invalid service") {
		t.Fatalf("bad: %v", err)
	}

	// Index is not updated if nothing is saved.
	if idx := s.maxIndex("queries"); idx != 0 {
		t.Fatalf("bad index: %d", idx)
	}

	// Now register the service.
	testRegisterNode(t, s, 1, "foo")
	testRegisterService(t, s, 2, "foo", "redis")

	// This should go through.
	if err := s.QuerySet(3, query); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure the index got updated.
	if idx := s.maxIndex("queries"); idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}

	// Read it back out and verify it.
	expected := &structs.PreparedQuery{
		ID: query.ID,
		Service: structs.ServiceQuery{
			Service: "redis",
		},
		RaftIndex: structs.RaftIndex{
			CreateIndex: 3,
			ModifyIndex: 3,
		},
	}
	idx, actual, err := s.QueryGet(query.ID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %v", actual)
	}

	// Give it a name and set it again.
	query.Name = "test-query"
	if err := s.QuerySet(4, query); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure the index got updated.
	if idx := s.maxIndex("queries"); idx != 4 {
		t.Fatalf("bad index: %d", idx)
	}

	// Read it back and verify the data was updated as well as the index.
	expected.Name = "test-query"
	expected.ModifyIndex = 4
	idx, actual, err = s.QueryGet(query.ID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 4 {
		t.Fatalf("bad index: %d", idx)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %v", actual)
	}

	// Try to tie it to a bogus session.
	query.Session = testUUID()
	err = s.QuerySet(5, query)
	if err == nil || !strings.Contains(err.Error(), "invalid session") {
		t.Fatalf("bad: %v", err)
	}

	// Index is not updated if nothing is saved.
	if idx := s.maxIndex("queries"); idx != 4 {
		t.Fatalf("bad index: %d", idx)
	}

	// Now make a session and try again.
	session := &structs.Session{
		ID:   query.Session,
		Node: "foo",
	}
	if err := s.SessionCreate(5, session); err != nil {
		t.Fatalf("err: %s", err)
	}
	if err := s.QuerySet(6, query); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure the index got updated.
	if idx := s.maxIndex("queries"); idx != 6 {
		t.Fatalf("bad index: %d", idx)
	}

	// Read it back and verify the data was updated as well as the index.
	expected.Session = query.Session
	expected.ModifyIndex = 6
	idx, actual, err = s.QueryGet(query.ID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 6 {
		t.Fatalf("bad index: %d", idx)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %v", actual)
	}

	// Finally, try to abuse the system by trying to register a query whose
	// name aliases a real query ID.
	evil := &structs.PreparedQuery{
		ID:   testUUID(),
		Name: query.ID,
		Service: structs.ServiceQuery{
			Service: "redis",
		},
	}
	err = s.QuerySet(7, evil)
	if err == nil || !strings.Contains(err.Error(), "aliases an existing query") {
		t.Fatalf("bad: %v", err)
	}

	// Index is not updated if nothing is saved.
	if idx := s.maxIndex("queries"); idx != 6 {
		t.Fatalf("bad index: %d", idx)
	}

	// Sanity check to make sure it's not there.
	idx, actual, err = s.QueryGet(evil.ID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 6 {
		t.Fatalf("bad index: %d", idx)
	}
	if actual != nil {
		t.Fatalf("bad: %v", actual)
	}
}

func TestStateStore_Query_QueryDelete(t *testing.T) {
	s := testStateStore(t)

	// Set up our test environment.
	testRegisterNode(t, s, 1, "foo")
	testRegisterService(t, s, 2, "foo", "redis")

	// Create a new query.
	query := &structs.PreparedQuery{
		ID: testUUID(),
		Service: structs.ServiceQuery{
			Service: "redis",
		},
	}

	// Deleting a query that doesn't exist should be a no-op.
	if err := s.QueryDelete(3, query.ID); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Index is not updated if nothing is saved.
	if idx := s.maxIndex("queries"); idx != 0 {
		t.Fatalf("bad index: %d", idx)
	}

	// Now add the query to the data store.
	if err := s.QuerySet(3, query); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure the index got updated.
	if idx := s.maxIndex("queries"); idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}

	// Read it back out and verify it.
	expected := &structs.PreparedQuery{
		ID: query.ID,
		Service: structs.ServiceQuery{
			Service: "redis",
		},
		RaftIndex: structs.RaftIndex{
			CreateIndex: 3,
			ModifyIndex: 3,
		},
	}
	idx, actual, err := s.QueryGet(query.ID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %v", actual)
	}

	// Now delete it.
	if err := s.QueryDelete(4, query.ID); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure the index got updated.
	if idx := s.maxIndex("queries"); idx != 4 {
		t.Fatalf("bad index: %d", idx)
	}

	// Sanity check to make sure it's not there.
	idx, actual, err = s.QueryGet(query.ID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 4 {
		t.Fatalf("bad index: %d", idx)
	}
	if actual != nil {
		t.Fatalf("bad: %v", actual)
	}
}

func TestStateStore_Query_QueryLookup(t *testing.T) {
	s := testStateStore(t)

	// Set up our test environment.
	testRegisterNode(t, s, 1, "foo")
	testRegisterService(t, s, 2, "foo", "redis")

	// Create a new query.
	query := &structs.PreparedQuery{
		ID:   testUUID(),
		Name: "my-test-query",
		Service: structs.ServiceQuery{
			Service: "redis",
		},
	}

	// Try to lookup a query that's not there using something that looks
	// like a real ID.
	idx, actual, err := s.QueryLookup(query.ID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 0 {
		t.Fatalf("bad index: %d", idx)
	}
	if actual != nil {
		t.Fatalf("bad: %v", actual)
	}

	// Try to lookup a query that's not there using something that looks
	// like a name
	idx, actual, err = s.QueryLookup(query.Name)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 0 {
		t.Fatalf("bad index: %d", idx)
	}
	if actual != nil {
		t.Fatalf("bad: %v", actual)
	}

	// Now actually insert the query.
	if err := s.QuerySet(3, query); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Make sure the index got updated.
	if idx := s.maxIndex("queries"); idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}

	// Read it back out using the ID and verify it.
	expected := &structs.PreparedQuery{
		ID:   query.ID,
		Name: "my-test-query",
		Service: structs.ServiceQuery{
			Service: "redis",
		},
		RaftIndex: structs.RaftIndex{
			CreateIndex: 3,
			ModifyIndex: 3,
		},
	}
	idx, actual, err = s.QueryLookup(query.ID)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %v", actual)
	}

	// Read it back using the name and verify it again.
	idx, actual, err = s.QueryLookup(query.Name)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 3 {
		t.Fatalf("bad index: %d", idx)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %v", actual)
	}

	// Make sure an empty lookup is well-behaved if there are actual queries
	// in the state store.
	if _, _, err = s.QueryLookup(""); err != ErrMissingQueryID {
		t.Fatalf("bad: %v", err)
	}
}

func TestStateStore_Query_QueryList(t *testing.T) {
	s := testStateStore(t)

	// Set up our test environment.
	testRegisterNode(t, s, 1, "foo")
	testRegisterService(t, s, 2, "foo", "redis")
	testRegisterService(t, s, 3, "foo", "mongodb")

	// Create some queries.
	queries := structs.PreparedQueries{
		&structs.PreparedQuery{
			ID:   testUUID(),
			Name: "alice",
			Service: structs.ServiceQuery{
				Service: "redis",
			},
		},
		&structs.PreparedQuery{
			ID:   testUUID(),
			Name: "bob",
			Service: structs.ServiceQuery{
				Service: "mongodb",
			},
		},
	}

	// Force the sort order of the UUIDs before we create them so the
	// order is deterministic.
	queries[0].ID = "a" + queries[0].ID[1:]
	queries[1].ID = "b" + queries[1].ID[1:]

	// Now create the queries.
	for i, query := range queries {
		if err := s.QuerySet(uint64(4+i), query); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Read it back and verify.
	expected := structs.PreparedQueries{
		&structs.PreparedQuery{
			ID:   queries[0].ID,
			Name: "alice",
			Service: structs.ServiceQuery{
				Service: "redis",
			},
			RaftIndex: structs.RaftIndex{
				CreateIndex: 4,
				ModifyIndex: 4,
			},
		},
		&structs.PreparedQuery{
			ID:   queries[1].ID,
			Name: "bob",
			Service: structs.ServiceQuery{
				Service: "mongodb",
			},
			RaftIndex: structs.RaftIndex{
				CreateIndex: 5,
				ModifyIndex: 5,
			},
		},
	}
	idx, actual, err := s.QueryList()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	if idx != 5 {
		t.Fatalf("bad index: %d", idx)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("bad: %v", actual)
	}
}

func TestStateStore_Query_Snapshot_Restore(t *testing.T) {
	s := testStateStore(t)

	// Set up our test environment.
	testRegisterNode(t, s, 1, "foo")
	testRegisterService(t, s, 2, "foo", "redis")
	testRegisterService(t, s, 3, "foo", "mongodb")

	// Create some queries.
	queries := structs.PreparedQueries{
		&structs.PreparedQuery{
			ID:   testUUID(),
			Name: "alice",
			Service: structs.ServiceQuery{
				Service: "redis",
			},
		},
		&structs.PreparedQuery{
			ID:   testUUID(),
			Name: "bob",
			Service: structs.ServiceQuery{
				Service: "mongodb",
			},
		},
	}

	// Force the sort order of the UUIDs before we create them so the
	// order is deterministic.
	queries[0].ID = "a" + queries[0].ID[1:]
	queries[1].ID = "b" + queries[1].ID[1:]

	// Now create the queries.
	for i, query := range queries {
		if err := s.QuerySet(uint64(4+i), query); err != nil {
			t.Fatalf("err: %s", err)
		}
	}

	// Snapshot the queries.
	snap := s.Snapshot()
	defer snap.Close()

	// Alter the real state store.
	if err := s.QueryDelete(6, queries[0].ID); err != nil {
		t.Fatalf("err: %s", err)
	}

	// Verify the snapshot.
	if idx := snap.LastIndex(); idx != 5 {
		t.Fatalf("bad index: %d", idx)
	}
	expected := structs.PreparedQueries{
		&structs.PreparedQuery{
			ID:   queries[0].ID,
			Name: "alice",
			Service: structs.ServiceQuery{
				Service: "redis",
			},
			RaftIndex: structs.RaftIndex{
				CreateIndex: 4,
				ModifyIndex: 4,
			},
		},
		&structs.PreparedQuery{
			ID:   queries[1].ID,
			Name: "bob",
			Service: structs.ServiceQuery{
				Service: "mongodb",
			},
			RaftIndex: structs.RaftIndex{
				CreateIndex: 5,
				ModifyIndex: 5,
			},
		},
	}
	iter, err := snap.Queries()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	var dump structs.PreparedQueries
	for query := iter.Next(); query != nil; query = iter.Next() {
		dump = append(dump, query.(*structs.PreparedQuery))
	}
	if !reflect.DeepEqual(dump, expected) {
		t.Fatalf("bad: %v", dump)
	}

	// Restore the values into a new state store.
	func() {
		s := testStateStore(t)
		restore := s.Restore()
		for _, query := range dump {
			if err := restore.Query(query); err != nil {
				t.Fatalf("err: %s", err)
			}
		}
		restore.Commit()

		// Read the restored queries back out and verify that they
		// match.
		idx, actual, err := s.QueryList()
		if err != nil {
			t.Fatalf("err: %s", err)
		}
		if idx != 5 {
			t.Fatalf("bad index: %d", idx)
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("bad: %v", actual)
		}
	}()
}

func TestStateStore_Query_Watches(t *testing.T) {
	s := testStateStore(t)

	// Set up our test environment.
	testRegisterNode(t, s, 1, "foo")
	testRegisterService(t, s, 2, "foo", "redis")

	query := &structs.PreparedQuery{
		ID: testUUID(),
		Service: structs.ServiceQuery{
			Service: "redis",
		},
	}

	// Call functions that update the queries table and make sure a watch
	// fires each time.
	verifyWatch(t, s.getTableWatch("queries"), func() {
		if err := s.QuerySet(3, query); err != nil {
			t.Fatalf("err: %s", err)
		}
	})
	verifyWatch(t, s.getTableWatch("queries"), func() {
		if err := s.QueryDelete(4, query.ID); err != nil {
			t.Fatalf("err: %s", err)
		}
	})
	verifyWatch(t, s.getTableWatch("queries"), func() {
		restore := s.Restore()
		if err := restore.Query(query); err != nil {
			t.Fatalf("err: %s", err)
		}
		restore.Commit()
	})
}
