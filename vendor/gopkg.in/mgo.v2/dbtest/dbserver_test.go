package dbtest_test

import (
	"os"
	"testing"
	"time"

	. "gopkg.in/check.v1"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/dbtest"
)

type M map[string]interface{}

func TestAll(t *testing.T) {
	TestingT(t)
}

type S struct {
	oldCheckSessions string
}

var _ = Suite(&S{})

func (s *S) SetUpTest(c *C) {
	s.oldCheckSessions = os.Getenv("CHECK_SESSIONS")
	os.Setenv("CHECK_SESSIONS", "")
}

func (s *S) TearDownTest(c *C) {
	os.Setenv("CHECK_SESSIONS", s.oldCheckSessions)
}

func (s *S) TestWipeData(c *C) {
	var server dbtest.DBServer
	server.SetPath(c.MkDir())
	defer server.Stop()

	session := server.Session()
	err := session.DB("mydb").C("mycoll").Insert(M{"a": 1})
	session.Close()
	c.Assert(err, IsNil)

	server.Wipe()

	session = server.Session()
	names, err := session.DatabaseNames()
	session.Close()
	c.Assert(err, IsNil)
	for _, name := range names {
		if name != "local" && name != "admin" {
			c.Fatalf("Wipe should have removed this database: %s", name)
		}
	}
}

func (s *S) TestStop(c *C) {
	var server dbtest.DBServer
	server.SetPath(c.MkDir())
	defer server.Stop()

	// Server should not be running.
	process := server.ProcessTest()
	c.Assert(process, IsNil)

	session := server.Session()
	addr := session.LiveServers()[0]
	session.Close()

	// Server should be running now.
	process = server.ProcessTest()
	p, err := os.FindProcess(process.Pid)
	c.Assert(err, IsNil)
	p.Release()

	server.Stop()

	// Server should not be running anymore.
	session, err = mgo.DialWithTimeout(addr, 500*time.Millisecond)
	if session != nil {
		session.Close()
		c.Fatalf("Stop did not stop the server")
	}
}

func (s *S) TestCheckSessions(c *C) {
	var server dbtest.DBServer
	server.SetPath(c.MkDir())
	defer server.Stop()

	session := server.Session()
	defer session.Close()
	c.Assert(server.Wipe, PanicMatches, "There are mgo sessions still alive.")
}

func (s *S) TestCheckSessionsDisabled(c *C) {
	var server dbtest.DBServer
	server.SetPath(c.MkDir())
	defer server.Stop()

	os.Setenv("CHECK_SESSIONS", "0")

	// Should not panic, although it looks to Wipe like this session will leak.
	session := server.Session()
	defer session.Close()
	server.Wipe()
}
