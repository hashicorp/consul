package dbtest

import (
	"os"
)

func (dbs *DBServer) ProcessTest() *os.Process {
	if dbs.server == nil {
		return nil
	}
	return dbs.server.Process
}
