// Package storage abstracts away where middleware can store assests (zones, keys, etc).
package storage

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
)

// dir wraps an http.Dir that restrict file access to a specific directory tree, see http.Dir's documentation
// for methods for accessing files.
type dir http.Dir

// CoreDir is the directory where middleware can store assets, like zone files after a zone transfer
// or public and private keys or anything else a middleware might need. The convention is to place
// assets in a subdirectory named after the zone prefixed with "D", to prevent the root zone become a hidden directory.
//
// Dexample.org/Kexample.org<something>.key
//
// Note that subzone(s) under example.org are places in the own directory under CoreDir:
//
// Dexample.org/...
// Db.example.org/...
//
// CoreDir will default to "$HOME/.coredns" on Unix, but it's location can be overriden with the COREDNSPATH
// environment variable.
var CoreDir = dir(fsPath())

func (d dir) Zone(z string) dir {
	if z != "." && z[len(z)-2] == '.' {
		return dir(path.Join(string(d), "D"+z[:len(z)-1]))
	}
	return dir(path.Join(string(d), "D"+z))
}

// fsPath returns the path to the directory where the application may store data.
// If COREDNSPATH env variable. is set, that value is used. Otherwise, the path is
// the result of evaluating "$HOME/.coredns".
func fsPath() string {
	if corePath := os.Getenv("COREDNSPATH"); corePath != "" {
		return corePath
	}
	return filepath.Join(userHomeDir(), ".coredns")
}

// userHomeDir returns the user's home directory according to environment variables.
//
// Credit: http://stackoverflow.com/a/7922977/1048862
func userHomeDir() string {
	if runtime.GOOS == "windows" {
		home := os.Getenv("HOMEDRIVE") + os.Getenv("HOMEPATH")
		if home == "" {
			home = os.Getenv("USERPROFILE")
		}
		return home
	}
	return os.Getenv("HOME")
}
