package txn_test

import (
	"flag"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"gopkg.in/mgo.v2/dbtest"
	"gopkg.in/mgo.v2/txn"
	. "gopkg.in/check.v1"
	"math/rand"
	"time"
)

var (
	duration = flag.Duration("duration", 200*time.Millisecond, "duration for each simulation")
	seed     = flag.Int64("seed", 0, "seed for rand")
)

type params struct {
	killChance     float64
	slowdownChance float64
	slowdown       time.Duration

	unsafe         bool
	workers        int
	accounts       int
	changeHalf     bool
	reinsertCopy   bool
	reinsertZeroed bool
	changelog      bool

	changes int
}

func (s *S) TestSim1Worker(c *C) {
	simulate(c, &s.server, params{
		workers:        1,
		accounts:       4,
		killChance:     0.01,
		slowdownChance: 0.3,
		slowdown:       100 * time.Millisecond,
	})
}

func (s *S) TestSim4WorkersDense(c *C) {
	simulate(c, &s.server, params{
		workers:        4,
		accounts:       2,
		killChance:     0.01,
		slowdownChance: 0.3,
		slowdown:       100 * time.Millisecond,
	})
}

func (s *S) TestSim4WorkersSparse(c *C) {
	simulate(c, &s.server, params{
		workers:        4,
		accounts:       10,
		killChance:     0.01,
		slowdownChance: 0.3,
		slowdown:       100 * time.Millisecond,
	})
}

func (s *S) TestSimHalf1Worker(c *C) {
	simulate(c, &s.server, params{
		workers:        1,
		accounts:       4,
		changeHalf:     true,
		killChance:     0.01,
		slowdownChance: 0.3,
		slowdown:       100 * time.Millisecond,
	})
}

func (s *S) TestSimHalf4WorkersDense(c *C) {
	simulate(c, &s.server, params{
		workers:        4,
		accounts:       2,
		changeHalf:     true,
		killChance:     0.01,
		slowdownChance: 0.3,
		slowdown:       100 * time.Millisecond,
	})
}

func (s *S) TestSimHalf4WorkersSparse(c *C) {
	simulate(c, &s.server, params{
		workers:        4,
		accounts:       10,
		changeHalf:     true,
		killChance:     0.01,
		slowdownChance: 0.3,
		slowdown:       100 * time.Millisecond,
	})
}

func (s *S) TestSimReinsertCopy1Worker(c *C) {
	simulate(c, &s.server, params{
		workers:        1,
		accounts:       10,
		reinsertCopy:   true,
		killChance:     0.01,
		slowdownChance: 0.3,
		slowdown:       100 * time.Millisecond,
	})
}

func (s *S) TestSimReinsertCopy4Workers(c *C) {
	simulate(c, &s.server, params{
		workers:        4,
		accounts:       10,
		reinsertCopy:   true,
		killChance:     0.01,
		slowdownChance: 0.3,
		slowdown:       100 * time.Millisecond,
	})
}

func (s *S) TestSimReinsertZeroed1Worker(c *C) {
	simulate(c, &s.server, params{
		workers:        1,
		accounts:       10,
		reinsertZeroed: true,
		killChance:     0.01,
		slowdownChance: 0.3,
		slowdown:       100 * time.Millisecond,
	})
}

func (s *S) TestSimReinsertZeroed4Workers(c *C) {
	simulate(c, &s.server, params{
		workers:        4,
		accounts:       10,
		reinsertZeroed: true,
		killChance:     0.01,
		slowdownChance: 0.3,
		slowdown:       100 * time.Millisecond,
	})
}

func (s *S) TestSimChangeLog(c *C) {
	simulate(c, &s.server, params{
		workers:        4,
		accounts:       10,
		killChance:     0.01,
		slowdownChance: 0.3,
		slowdown:       100 * time.Millisecond,
		changelog:      true,
	})
}

type balanceChange struct {
	id     bson.ObjectId
	origin int
	target int
	amount int
}

func simulate(c *C, server *dbtest.DBServer, params params) {
	seed := *seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rand.Seed(seed)
	c.Logf("Seed: %v", seed)

	txn.SetChaos(txn.Chaos{
		KillChance:     params.killChance,
		SlowdownChance: params.slowdownChance,
		Slowdown:       params.slowdown,
	})
	defer txn.SetChaos(txn.Chaos{})

	session := server.Session()
	defer session.Close()

	db := session.DB("test")
	tc := db.C("tc")

	runner := txn.NewRunner(tc)

	tclog := db.C("tc.log")
	if params.changelog {
		info := mgo.CollectionInfo{
			Capped:   true,
			MaxBytes: 1000000,
		}
		err := tclog.Create(&info)
		c.Assert(err, IsNil)
		runner.ChangeLog(tclog)
	}

	accounts := db.C("accounts")
	for i := 0; i < params.accounts; i++ {
		err := accounts.Insert(M{"_id": i, "balance": 300})
		c.Assert(err, IsNil)
	}
	var stop time.Time
	if params.changes <= 0 {
		stop = time.Now().Add(*duration)
	}

	max := params.accounts
	if params.reinsertCopy || params.reinsertZeroed {
		max = int(float64(params.accounts) * 1.5)
	}

	changes := make(chan balanceChange, 1024)

	//session.SetMode(mgo.Eventual, true)
	for i := 0; i < params.workers; i++ {
		go func() {
			n := 0
			for {
				if n > 0 && n == params.changes {
					break
				}
				if !stop.IsZero() && time.Now().After(stop) {
					break
				}

				change := balanceChange{
					id:     bson.NewObjectId(),
					origin: rand.Intn(max),
					target: rand.Intn(max),
					amount: 100,
				}

				var old Account
				var oldExists bool
				if params.reinsertCopy || params.reinsertZeroed {
					if err := accounts.FindId(change.origin).One(&old); err != mgo.ErrNotFound {
						c.Check(err, IsNil)
						change.amount = old.Balance
						oldExists = true
					}
				}

				var ops []txn.Op
				switch {
				case params.reinsertCopy && oldExists:
					ops = []txn.Op{{
						C:      "accounts",
						Id:     change.origin,
						Assert: M{"balance": change.amount},
						Remove: true,
					}, {
						C:      "accounts",
						Id:     change.target,
						Assert: txn.DocMissing,
						Insert: M{"balance": change.amount},
					}}
				case params.reinsertZeroed && oldExists:
					ops = []txn.Op{{
						C:      "accounts",
						Id:     change.target,
						Assert: txn.DocMissing,
						Insert: M{"balance": 0},
					}, {
						C:      "accounts",
						Id:     change.origin,
						Assert: M{"balance": change.amount},
						Remove: true,
					}, {
						C:      "accounts",
						Id:     change.target,
						Assert: txn.DocExists,
						Update: M{"$inc": M{"balance": change.amount}},
					}}
				case params.changeHalf:
					ops = []txn.Op{{
						C:      "accounts",
						Id:     change.origin,
						Assert: M{"balance": M{"$gte": change.amount}},
						Update: M{"$inc": M{"balance": -change.amount / 2}},
					}, {
						C:      "accounts",
						Id:     change.target,
						Assert: txn.DocExists,
						Update: M{"$inc": M{"balance": change.amount / 2}},
					}, {
						C:      "accounts",
						Id:     change.origin,
						Update: M{"$inc": M{"balance": -change.amount / 2}},
					}, {
						C:      "accounts",
						Id:     change.target,
						Update: M{"$inc": M{"balance": change.amount / 2}},
					}}
				default:
					ops = []txn.Op{{
						C:      "accounts",
						Id:     change.origin,
						Assert: M{"balance": M{"$gte": change.amount}},
						Update: M{"$inc": M{"balance": -change.amount}},
					}, {
						C:      "accounts",
						Id:     change.target,
						Assert: txn.DocExists,
						Update: M{"$inc": M{"balance": change.amount}},
					}}
				}

				err := runner.Run(ops, change.id, nil)
				if err != nil && err != txn.ErrAborted && err != txn.ErrChaos {
					c.Check(err, IsNil)
				}
				n++
				changes <- change
			}
			changes <- balanceChange{}
		}()
	}

	alive := params.workers
	changeLog := make([]balanceChange, 0, 1024)
	for alive > 0 {
		change := <-changes
		if change.id == "" {
			alive--
		} else {
			changeLog = append(changeLog, change)
		}
	}
	c.Check(len(changeLog), Not(Equals), 0, Commentf("No operations were even attempted."))

	txn.SetChaos(txn.Chaos{})
	err := runner.ResumeAll()
	c.Assert(err, IsNil)

	n, err := accounts.Count()
	c.Check(err, IsNil)
	c.Check(n, Equals, params.accounts, Commentf("Number of accounts has changed."))

	n, err = accounts.Find(M{"balance": M{"$lt": 0}}).Count()
	c.Check(err, IsNil)
	c.Check(n, Equals, 0, Commentf("There are %d accounts with negative balance.", n))

	globalBalance := 0
	iter := accounts.Find(nil).Iter()
	account := Account{}
	for iter.Next(&account) {
		globalBalance += account.Balance
	}
	c.Check(iter.Close(), IsNil)
	c.Check(globalBalance, Equals, params.accounts*300, Commentf("Total amount of money should be constant."))

	// Compute and verify the exact final state of all accounts.
	balance := make(map[int]int)
	for i := 0; i < params.accounts; i++ {
		balance[i] += 300
	}
	var applied, aborted int
	for _, change := range changeLog {
		err := runner.Resume(change.id)
		if err == txn.ErrAborted {
			aborted++
			continue
		} else if err != nil {
			c.Fatalf("resuming %s failed: %v", change.id, err)
		}
		balance[change.origin] -= change.amount
		balance[change.target] += change.amount
		applied++
	}
	iter = accounts.Find(nil).Iter()
	for iter.Next(&account) {
		c.Assert(account.Balance, Equals, balance[account.Id])
	}
	c.Check(iter.Close(), IsNil)
	c.Logf("Total transactions: %d (%d applied, %d aborted)", len(changeLog), applied, aborted)

	if params.changelog {
		n, err := tclog.Count()
		c.Assert(err, IsNil)
		// Check if the capped collection is full.
		dummy := make([]byte, 1024)
		tclog.Insert(M{"_id": bson.NewObjectId(), "dummy": dummy})
		m, err := tclog.Count()
		c.Assert(err, IsNil)
		if m == n+1 {
			// Wasn't full, so it must have seen it all.
			c.Assert(err, IsNil)
			c.Assert(n, Equals, applied)
		}
	}
}
