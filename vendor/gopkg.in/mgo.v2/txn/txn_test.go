package txn_test

import (
	"flag"
	"fmt"
	"sync"
	"testing"
	"time"

	. "gopkg.in/check.v1"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/mgo.v2/dbtest"
	"gopkg.in/mgo.v2/txn"
)

func TestAll(t *testing.T) {
	TestingT(t)
}

type S struct {
	server   dbtest.DBServer
	session  *mgo.Session
	db       *mgo.Database
	tc, sc   *mgo.Collection
	accounts *mgo.Collection
	runner   *txn.Runner
}

var _ = Suite(&S{})

type M map[string]interface{}

func (s *S) SetUpSuite(c *C) {
	s.server.SetPath(c.MkDir())
}

func (s *S) TearDownSuite(c *C) {
	s.server.Stop()
}

func (s *S) SetUpTest(c *C) {
	s.server.Wipe()

	txn.SetChaos(txn.Chaos{})
	txn.SetLogger(c)
	txn.SetDebug(true)

	s.session = s.server.Session()
	s.db = s.session.DB("test")
	s.tc = s.db.C("tc")
	s.sc = s.db.C("tc.stash")
	s.accounts = s.db.C("accounts")
	s.runner = txn.NewRunner(s.tc)
}

func (s *S) TearDownTest(c *C) {
	txn.SetLogger(nil)
	txn.SetDebug(false)
	s.session.Close()
}

type Account struct {
	Id      int `bson:"_id"`
	Balance int
}

func (s *S) TestDocExists(c *C) {
	err := s.accounts.Insert(M{"_id": 0, "balance": 300})
	c.Assert(err, IsNil)

	exists := []txn.Op{{
		C:      "accounts",
		Id:     0,
		Assert: txn.DocExists,
	}}
	missing := []txn.Op{{
		C:      "accounts",
		Id:     0,
		Assert: txn.DocMissing,
	}}

	err = s.runner.Run(exists, "", nil)
	c.Assert(err, IsNil)
	err = s.runner.Run(missing, "", nil)
	c.Assert(err, Equals, txn.ErrAborted)

	err = s.accounts.RemoveId(0)
	c.Assert(err, IsNil)

	err = s.runner.Run(exists, "", nil)
	c.Assert(err, Equals, txn.ErrAborted)
	err = s.runner.Run(missing, "", nil)
	c.Assert(err, IsNil)
}

func (s *S) TestInsert(c *C) {
	err := s.accounts.Insert(M{"_id": 0, "balance": 300})
	c.Assert(err, IsNil)

	ops := []txn.Op{{
		C:      "accounts",
		Id:     0,
		Insert: M{"balance": 200},
	}}

	err = s.runner.Run(ops, "", nil)
	c.Assert(err, IsNil)

	var account Account
	err = s.accounts.FindId(0).One(&account)
	c.Assert(err, IsNil)
	c.Assert(account.Balance, Equals, 300)

	ops[0].Id = 1
	err = s.runner.Run(ops, "", nil)
	c.Assert(err, IsNil)

	err = s.accounts.FindId(1).One(&account)
	c.Assert(err, IsNil)
	c.Assert(account.Balance, Equals, 200)
}

func (s *S) TestInsertStructID(c *C) {
	type id struct {
		FirstName string
		LastName  string
	}
	ops := []txn.Op{{
		C:      "accounts",
		Id:     id{FirstName: "John", LastName: "Jones"},
		Assert: txn.DocMissing,
		Insert: M{"balance": 200},
	}, {
		C:      "accounts",
		Id:     id{FirstName: "Sally", LastName: "Smith"},
		Assert: txn.DocMissing,
		Insert: M{"balance": 800},
	}}

	err := s.runner.Run(ops, "", nil)
	c.Assert(err, IsNil)

	n, err := s.accounts.Find(nil).Count()
	c.Assert(err, IsNil)
	c.Assert(n, Equals, 2)
}

func (s *S) TestRemove(c *C) {
	err := s.accounts.Insert(M{"_id": 0, "balance": 300})
	c.Assert(err, IsNil)

	ops := []txn.Op{{
		C:      "accounts",
		Id:     0,
		Remove: true,
	}}

	err = s.runner.Run(ops, "", nil)
	c.Assert(err, IsNil)

	err = s.accounts.FindId(0).One(nil)
	c.Assert(err, Equals, mgo.ErrNotFound)

	err = s.runner.Run(ops, "", nil)
	c.Assert(err, IsNil)
}

func (s *S) TestUpdate(c *C) {
	var err error
	err = s.accounts.Insert(M{"_id": 0, "balance": 200})
	c.Assert(err, IsNil)
	err = s.accounts.Insert(M{"_id": 1, "balance": 200})
	c.Assert(err, IsNil)

	ops := []txn.Op{{
		C:      "accounts",
		Id:     0,
		Update: M{"$inc": M{"balance": 100}},
	}}

	err = s.runner.Run(ops, "", nil)
	c.Assert(err, IsNil)

	var account Account
	err = s.accounts.FindId(0).One(&account)
	c.Assert(err, IsNil)
	c.Assert(account.Balance, Equals, 300)

	ops[0].Id = 1

	err = s.accounts.FindId(1).One(&account)
	c.Assert(err, IsNil)
	c.Assert(account.Balance, Equals, 200)
}

func (s *S) TestInsertUpdate(c *C) {
	ops := []txn.Op{{
		C:      "accounts",
		Id:     0,
		Insert: M{"_id": 0, "balance": 200},
	}, {
		C:      "accounts",
		Id:     0,
		Update: M{"$inc": M{"balance": 100}},
	}}

	err := s.runner.Run(ops, "", nil)
	c.Assert(err, IsNil)

	var account Account
	err = s.accounts.FindId(0).One(&account)
	c.Assert(err, IsNil)
	c.Assert(account.Balance, Equals, 300)

	err = s.runner.Run(ops, "", nil)
	c.Assert(err, IsNil)

	err = s.accounts.FindId(0).One(&account)
	c.Assert(err, IsNil)
	c.Assert(account.Balance, Equals, 400)
}

func (s *S) TestUpdateInsert(c *C) {
	ops := []txn.Op{{
		C:      "accounts",
		Id:     0,
		Update: M{"$inc": M{"balance": 100}},
	}, {
		C:      "accounts",
		Id:     0,
		Insert: M{"_id": 0, "balance": 200},
	}}

	err := s.runner.Run(ops, "", nil)
	c.Assert(err, IsNil)

	var account Account
	err = s.accounts.FindId(0).One(&account)
	c.Assert(err, IsNil)
	c.Assert(account.Balance, Equals, 200)

	err = s.runner.Run(ops, "", nil)
	c.Assert(err, IsNil)

	err = s.accounts.FindId(0).One(&account)
	c.Assert(err, IsNil)
	c.Assert(account.Balance, Equals, 300)
}

func (s *S) TestInsertRemoveInsert(c *C) {
	ops := []txn.Op{{
		C:      "accounts",
		Id:     0,
		Insert: M{"_id": 0, "balance": 200},
	}, {
		C:      "accounts",
		Id:     0,
		Remove: true,
	}, {
		C:      "accounts",
		Id:     0,
		Insert: M{"_id": 0, "balance": 300},
	}}

	err := s.runner.Run(ops, "", nil)
	c.Assert(err, IsNil)

	var account Account
	err = s.accounts.FindId(0).One(&account)
	c.Assert(err, IsNil)
	c.Assert(account.Balance, Equals, 300)
}

func (s *S) TestQueueStashing(c *C) {
	txn.SetChaos(txn.Chaos{
		KillChance: 1,
		Breakpoint: "set-applying",
	})

	opses := [][]txn.Op{{{
		C:      "accounts",
		Id:     0,
		Insert: M{"balance": 100},
	}}, {{
		C:      "accounts",
		Id:     0,
		Remove: true,
	}}, {{
		C:      "accounts",
		Id:     0,
		Insert: M{"balance": 200},
	}}, {{
		C:      "accounts",
		Id:     0,
		Update: M{"$inc": M{"balance": 100}},
	}}}

	var last bson.ObjectId
	for _, ops := range opses {
		last = bson.NewObjectId()
		err := s.runner.Run(ops, last, nil)
		c.Assert(err, Equals, txn.ErrChaos)
	}

	txn.SetChaos(txn.Chaos{})
	err := s.runner.Resume(last)
	c.Assert(err, IsNil)

	var account Account
	err = s.accounts.FindId(0).One(&account)
	c.Assert(err, IsNil)
	c.Assert(account.Balance, Equals, 300)
}

func (s *S) TestInfo(c *C) {
	ops := []txn.Op{{
		C:      "accounts",
		Id:     0,
		Assert: txn.DocMissing,
	}}

	id := bson.NewObjectId()
	err := s.runner.Run(ops, id, M{"n": 42})
	c.Assert(err, IsNil)

	var t struct{ I struct{ N int } }
	err = s.tc.FindId(id).One(&t)
	c.Assert(err, IsNil)
	c.Assert(t.I.N, Equals, 42)
}

func (s *S) TestErrors(c *C) {
	doc := bson.M{"foo": 1}
	tests := []txn.Op{{
		C:  "c",
		Id: 0,
	}, {
		C:      "c",
		Id:     0,
		Insert: doc,
		Remove: true,
	}, {
		C:      "c",
		Id:     0,
		Insert: doc,
		Update: doc,
	}, {
		C:      "c",
		Id:     0,
		Update: doc,
		Remove: true,
	}, {
		C:      "c",
		Assert: doc,
	}, {
		Id:     0,
		Assert: doc,
	}}

	txn.SetChaos(txn.Chaos{KillChance: 1.0})
	for _, op := range tests {
		c.Logf("op: %v", op)
		err := s.runner.Run([]txn.Op{op}, "", nil)
		c.Assert(err, ErrorMatches, "error in transaction op 0: .*")
	}
}

func (s *S) TestAssertNestedOr(c *C) {
	// Assert uses $or internally. Ensure nesting works.
	err := s.accounts.Insert(M{"_id": 0, "balance": 300})
	c.Assert(err, IsNil)

	ops := []txn.Op{{
		C:      "accounts",
		Id:     0,
		Assert: bson.D{{"$or", []bson.D{{{"balance", 100}}, {{"balance", 300}}}}},
		Update: bson.D{{"$inc", bson.D{{"balance", 100}}}},
	}}

	err = s.runner.Run(ops, "", nil)
	c.Assert(err, IsNil)

	var account Account
	err = s.accounts.FindId(0).One(&account)
	c.Assert(err, IsNil)
	c.Assert(account.Balance, Equals, 400)
}

func (s *S) TestVerifyFieldOrdering(c *C) {
	// Used to have a map in certain operations, which means
	// the ordering of fields would be messed up.
	fields := bson.D{{"a", 1}, {"b", 2}, {"c", 3}}
	ops := []txn.Op{{
		C:      "accounts",
		Id:     0,
		Insert: fields,
	}}

	err := s.runner.Run(ops, "", nil)
	c.Assert(err, IsNil)

	var d bson.D
	err = s.accounts.FindId(0).One(&d)
	c.Assert(err, IsNil)

	var filtered bson.D
	for _, e := range d {
		switch e.Name {
		case "a", "b", "c":
			filtered = append(filtered, e)
		}
	}
	c.Assert(filtered, DeepEquals, fields)
}

func (s *S) TestChangeLog(c *C) {
	chglog := s.db.C("chglog")
	s.runner.ChangeLog(chglog)

	ops := []txn.Op{{
		C:      "debts",
		Id:     0,
		Assert: txn.DocMissing,
	}, {
		C:      "accounts",
		Id:     0,
		Insert: M{"balance": 300},
	}, {
		C:      "accounts",
		Id:     1,
		Insert: M{"balance": 300},
	}, {
		C:      "people",
		Id:     "joe",
		Insert: M{"accounts": []int64{0, 1}},
	}}
	id := bson.NewObjectId()
	err := s.runner.Run(ops, id, nil)
	c.Assert(err, IsNil)

	type IdList []interface{}
	type Log struct {
		Docs   IdList  "d"
		Revnos []int64 "r"
	}
	var m map[string]*Log
	err = chglog.FindId(id).One(&m)
	c.Assert(err, IsNil)

	c.Assert(m["accounts"], DeepEquals, &Log{IdList{0, 1}, []int64{2, 2}})
	c.Assert(m["people"], DeepEquals, &Log{IdList{"joe"}, []int64{2}})
	c.Assert(m["debts"], IsNil)

	ops = []txn.Op{{
		C:      "accounts",
		Id:     0,
		Update: M{"$inc": M{"balance": 100}},
	}, {
		C:      "accounts",
		Id:     1,
		Update: M{"$inc": M{"balance": 100}},
	}}
	id = bson.NewObjectId()
	err = s.runner.Run(ops, id, nil)
	c.Assert(err, IsNil)

	m = nil
	err = chglog.FindId(id).One(&m)
	c.Assert(err, IsNil)

	c.Assert(m["accounts"], DeepEquals, &Log{IdList{0, 1}, []int64{3, 3}})
	c.Assert(m["people"], IsNil)

	ops = []txn.Op{{
		C:      "accounts",
		Id:     0,
		Remove: true,
	}, {
		C:      "people",
		Id:     "joe",
		Remove: true,
	}}
	id = bson.NewObjectId()
	err = s.runner.Run(ops, id, nil)
	c.Assert(err, IsNil)

	m = nil
	err = chglog.FindId(id).One(&m)
	c.Assert(err, IsNil)

	c.Assert(m["accounts"], DeepEquals, &Log{IdList{0}, []int64{-4}})
	c.Assert(m["people"], DeepEquals, &Log{IdList{"joe"}, []int64{-3}})
}

func (s *S) TestPurgeMissing(c *C) {
	txn.SetChaos(txn.Chaos{
		KillChance: 1,
		Breakpoint: "set-applying",
	})

	err := s.accounts.Insert(M{"_id": 0, "balance": 100})
	c.Assert(err, IsNil)
	err = s.accounts.Insert(M{"_id": 1, "balance": 100})
	c.Assert(err, IsNil)

	ops1 := []txn.Op{{
		C:      "accounts",
		Id:     3,
		Insert: M{"balance": 100},
	}}

	ops2 := []txn.Op{{
		C:      "accounts",
		Id:     0,
		Remove: true,
	}, {
		C:      "accounts",
		Id:     1,
		Update: M{"$inc": M{"balance": 100}},
	}, {
		C:      "accounts",
		Id:     2,
		Insert: M{"balance": 100},
	}}

	first := bson.NewObjectId()
	c.Logf("---- Running ops1 under transaction %q, to be canceled by chaos", first.Hex())
	err = s.runner.Run(ops1, first, nil)
	c.Assert(err, Equals, txn.ErrChaos)

	last := bson.NewObjectId()
	c.Logf("---- Running ops2 under transaction %q, to be canceled by chaos", last.Hex())
	err = s.runner.Run(ops2, last, nil)
	c.Assert(err, Equals, txn.ErrChaos)

	c.Logf("---- Removing transaction %q", last.Hex())
	err = s.tc.RemoveId(last)
	c.Assert(err, IsNil)

	c.Logf("---- Disabling chaos and attempting to resume all")
	txn.SetChaos(txn.Chaos{})
	err = s.runner.ResumeAll()
	c.Assert(err, IsNil)

	again := bson.NewObjectId()
	c.Logf("---- Running ops2 again under transaction %q, to fail for missing transaction", again.Hex())
	err = s.runner.Run(ops2, again, nil)
	c.Assert(err, ErrorMatches, "cannot find transaction .*")

	c.Logf("---- Purging missing transactions")
	err = s.runner.PurgeMissing("accounts")
	c.Assert(err, IsNil)

	c.Logf("---- Resuming pending transactions")
	err = s.runner.ResumeAll()
	c.Assert(err, IsNil)

	expect := []struct{ Id, Balance int }{
		{0, -1},
		{1, 200},
		{2, 100},
		{3, 100},
	}
	var got Account
	for _, want := range expect {
		err = s.accounts.FindId(want.Id).One(&got)
		if want.Balance == -1 {
			if err != mgo.ErrNotFound {
				c.Errorf("Account %d should not exist, find got err=%#v", err)
			}
		} else if err != nil {
			c.Errorf("Account %d should have balance of %d, but wasn't found", want.Id, want.Balance)
		} else if got.Balance != want.Balance {
			c.Errorf("Account %d should have balance of %d, got %d", want.Id, want.Balance, got.Balance)
		}
	}
}

func (s *S) TestTxnQueueStashStressTest(c *C) {
	txn.SetChaos(txn.Chaos{
		SlowdownChance: 0.3,
		Slowdown:       50 * time.Millisecond,
	})
	defer txn.SetChaos(txn.Chaos{})

	// So we can run more iterations of the test in less time.
	txn.SetDebug(false)

	const runners = 10
	const inserts = 10
	const repeat = 100

	for r := 0; r < repeat; r++ {
		var wg sync.WaitGroup
		wg.Add(runners)
		for i := 0; i < runners; i++ {
			go func(i, r int) {
				defer wg.Done()

				session := s.session.New()
				defer session.Close()
				runner := txn.NewRunner(s.tc.With(session))

				for j := 0; j < inserts; j++ {
					ops := []txn.Op{{
						C:  "accounts",
						Id: fmt.Sprintf("insert-%d-%d", r, j),
						Insert: bson.M{
							"added-by": i,
						},
					}}
					err := runner.Run(ops, "", nil)
					if err != txn.ErrAborted {
						c.Check(err, IsNil)
					}
				}
			}(i, r)
		}
		wg.Wait()
	}
}

func (s *S) TestPurgeMissingPipelineSizeLimit(c *C) {
	// This test ensures that PurgeMissing can handle very large
	// txn-queue fields. Previous iterations of PurgeMissing would
	// trigger a 16MB aggregation pipeline result size limit when run
	// against a documents or stashes with large numbers of txn-queue
	// entries. PurgeMissing now no longer uses aggregation pipelines
	// to work around this limit.

	// The pipeline result size limitation was removed from MongoDB in 2.6 so
	// this test is only run for older MongoDB version.
	build, err := s.session.BuildInfo()
	c.Assert(err, IsNil)
	if build.VersionAtLeast(2, 6) {
		c.Skip("This tests a problem that can only happen with MongoDB < 2.6 ")
	}

	// Insert a single document to work with.
	err = s.accounts.Insert(M{"_id": 0, "balance": 100})
	c.Assert(err, IsNil)

	ops := []txn.Op{{
		C:      "accounts",
		Id:     0,
		Update: M{"$inc": M{"balance": 100}},
	}}

	// Generate one successful transaction.
	good := bson.NewObjectId()
	c.Logf("---- Running ops under transaction %q", good.Hex())
	err = s.runner.Run(ops, good, nil)
	c.Assert(err, IsNil)

	// Generate another transaction which which will go missing.
	missing := bson.NewObjectId()
	c.Logf("---- Running ops under transaction %q (which will go missing)", missing.Hex())
	err = s.runner.Run(ops, missing, nil)
	c.Assert(err, IsNil)

	err = s.tc.RemoveId(missing)
	c.Assert(err, IsNil)

	// Generate a txn-queue on the test document that's large enough
	// that it used to cause PurgeMissing to exceed MongoDB's pipeline
	// result 16MB size limit (MongoDB 2.4 and older only).
	//
	// The contents of the txn-queue field doesn't matter, only that
	// it's big enough to trigger the size limit. The required size
	// can also be achieved by using multiple documents as long as the
	// cumulative size of all the txn-queue fields exceeds the
	// pipeline limit. A single document is easier to work with for
	// this test however.
	//
	// The txn id of the successful transaction is used fill the
	// txn-queue because this takes advantage of a short circuit in
	// PurgeMissing, dramatically speeding up the test run time.
	const fakeQueueLen = 250000
	fakeTxnQueue := make([]string, fakeQueueLen)
	token := good.Hex() + "_12345678" // txn id + nonce
	for i := 0; i < fakeQueueLen; i++ {
		fakeTxnQueue[i] = token
	}

	err = s.accounts.UpdateId(0, bson.M{
		"$set": bson.M{"txn-queue": fakeTxnQueue},
	})
	c.Assert(err, IsNil)

	// PurgeMissing could hit the same pipeline result size limit when
	// processing the txn-queue fields of stash documents so insert
	// the large txn-queue there too to ensure that no longer happens.
	err = s.sc.Insert(
		bson.D{{"c", "accounts"}, {"id", 0}},
		bson.M{"txn-queue": fakeTxnQueue},
	)
	c.Assert(err, IsNil)

	c.Logf("---- Purging missing transactions")
	err = s.runner.PurgeMissing("accounts")
	c.Assert(err, IsNil)
}

var flaky = flag.Bool("flaky", false, "Include flaky tests")

func (s *S) TestTxnQueueStressTest(c *C) {
	// This fails about 20% of the time on Mongo 3.2 (I haven't tried
	// other versions) with account balance being 3999 instead of
	// 4000. That implies that some updates are being lost. This is
	// bad and we'll need to chase it down in the near future - the
	// only reason it's being skipped now is that it's already failing
	// and it's better to have the txn tests running without this one
	// than to have them not running at all.
	if !*flaky {
		c.Skip("Fails intermittently - disabling until fixed")
	}
	txn.SetChaos(txn.Chaos{
		SlowdownChance: 0.3,
		Slowdown:       50 * time.Millisecond,
	})
	defer txn.SetChaos(txn.Chaos{})

	// So we can run more iterations of the test in less time.
	txn.SetDebug(false)

	err := s.accounts.Insert(M{"_id": 0, "balance": 0}, M{"_id": 1, "balance": 0})
	c.Assert(err, IsNil)

	// Run half of the operations changing account 0 and then 1,
	// and the other half in the opposite order.
	ops01 := []txn.Op{{
		C:      "accounts",
		Id:     0,
		Update: M{"$inc": M{"balance": 1}},
	}, {
		C:      "accounts",
		Id:     1,
		Update: M{"$inc": M{"balance": 1}},
	}}

	ops10 := []txn.Op{{
		C:      "accounts",
		Id:     1,
		Update: M{"$inc": M{"balance": 1}},
	}, {
		C:      "accounts",
		Id:     0,
		Update: M{"$inc": M{"balance": 1}},
	}}

	ops := [][]txn.Op{ops01, ops10}

	const runners = 4
	const changes = 1000

	var wg sync.WaitGroup
	wg.Add(runners)
	for n := 0; n < runners; n++ {
		n := n
		go func() {
			defer wg.Done()
			for i := 0; i < changes; i++ {
				err = s.runner.Run(ops[n%2], "", nil)
				c.Assert(err, IsNil)
			}
		}()
	}
	wg.Wait()

	for id := 0; id < 2; id++ {
		var account Account
		err = s.accounts.FindId(id).One(&account)
		if account.Balance != runners*changes {
			c.Errorf("Account should have balance of %d, got %d", runners*changes, account.Balance)
		}
	}
}
