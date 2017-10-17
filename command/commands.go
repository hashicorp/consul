package command

import (
	"os"
	"os/signal"
	"syscall"

	agentcmd "github.com/hashicorp/consul/command/agent"
	"github.com/hashicorp/consul/command/cat"
	"github.com/hashicorp/consul/command/catlistdc"
	"github.com/hashicorp/consul/command/catlistnodes"
	"github.com/hashicorp/consul/command/catlistsvc"
	"github.com/hashicorp/consul/command/event"
	execmd "github.com/hashicorp/consul/command/exec"
	"github.com/hashicorp/consul/command/forceleave"
	"github.com/hashicorp/consul/command/info"
	"github.com/hashicorp/consul/command/join"
	"github.com/hashicorp/consul/command/keygen"
	"github.com/hashicorp/consul/command/keyring"
	"github.com/hashicorp/consul/command/kv"
	"github.com/hashicorp/consul/command/kvdel"
	"github.com/hashicorp/consul/command/kvexp"
	"github.com/hashicorp/consul/command/kvget"
	"github.com/hashicorp/consul/command/kvimp"
	"github.com/hashicorp/consul/command/kvput"
	"github.com/hashicorp/consul/command/leave"
	"github.com/hashicorp/consul/command/lock"
	"github.com/hashicorp/consul/command/maint"
	"github.com/hashicorp/consul/command/members"
	"github.com/hashicorp/consul/command/monitor"
	"github.com/hashicorp/consul/command/oper"
	"github.com/hashicorp/consul/command/operauto"
	"github.com/hashicorp/consul/command/operautoget"
	"github.com/hashicorp/consul/command/operautoset"
	"github.com/hashicorp/consul/command/operraft"
	"github.com/hashicorp/consul/command/operraftlist"
	"github.com/hashicorp/consul/command/operraftremove"
	"github.com/hashicorp/consul/command/reload"
	"github.com/hashicorp/consul/command/rtt"
	"github.com/hashicorp/consul/command/snapshot"
	"github.com/hashicorp/consul/command/snapshotinspect"
	"github.com/hashicorp/consul/command/snapshotrestore"
	"github.com/hashicorp/consul/command/snapshotsave"
	"github.com/hashicorp/consul/command/validate"
	versioncmd "github.com/hashicorp/consul/command/version"
	watchcmd "github.com/hashicorp/consul/command/watch"
	"github.com/hashicorp/consul/version"
	"github.com/mitchellh/cli"
)

// Commands is the mapping of all the available Consul commands.
var Commands map[string]cli.CommandFactory

func init() {
	ui := &cli.BasicUi{Writer: os.Stdout, ErrorWriter: os.Stderr}

	Commands = map[string]cli.CommandFactory{
		"agent": func() (cli.Command, error) {
			return agentcmd.New(
				ui,
				version.GitCommit,
				version.Version,
				version.VersionPrerelease,
				version.GetHumanVersion(),
				make(chan struct{}),
			), nil
		},

		"catalog":                       func() (cli.Command, error) { return cat.New(), nil },
		"catalog datacenters":           func() (cli.Command, error) { return catlistdc.New(ui), nil },
		"catalog nodes":                 func() (cli.Command, error) { return catlistnodes.New(ui), nil },
		"catalog services":              func() (cli.Command, error) { return catlistsvc.New(ui), nil },
		"event":                         func() (cli.Command, error) { return event.New(ui), nil },
		"exec":                          func() (cli.Command, error) { return execmd.New(ui, makeShutdownCh()), nil },
		"force-leave":                   func() (cli.Command, error) { return forceleave.New(ui), nil },
		"info":                          func() (cli.Command, error) { return info.New(ui), nil },
		"join":                          func() (cli.Command, error) { return join.New(ui), nil },
		"keygen":                        func() (cli.Command, error) { return keygen.New(ui), nil },
		"keyring":                       func() (cli.Command, error) { return keyring.New(ui), nil },
		"kv delete":                     func() (cli.Command, error) { return kvdel.New(ui), nil },
		"kv export":                     func() (cli.Command, error) { return kvexp.New(ui), nil },
		"kv get":                        func() (cli.Command, error) { return kvget.New(ui), nil },
		"kv import":                     func() (cli.Command, error) { return kvimp.New(ui), nil },
		"kv put":                        func() (cli.Command, error) { return kvput.New(ui), nil },
		"kv":                            func() (cli.Command, error) { return kv.New(), nil },
		"leave":                         func() (cli.Command, error) { return leave.New(ui), nil },
		"lock":                          func() (cli.Command, error) { return lock.New(ui), nil },
		"maint":                         func() (cli.Command, error) { return maint.New(ui), nil },
		"members":                       func() (cli.Command, error) { return members.New(ui), nil },
		"monitor":                       func() (cli.Command, error) { return monitor.New(ui, makeShutdownCh()), nil },
		"operator":                      func() (cli.Command, error) { return oper.New(), nil },
		"operator autopilot":            func() (cli.Command, error) { return operauto.New(), nil },
		"operator autopilot get-config": func() (cli.Command, error) { return operautoget.New(ui), nil },
		"operator autopilot set-config": func() (cli.Command, error) { return operautoset.New(ui), nil },
		"operator raft":                 func() (cli.Command, error) { return operraft.New(), nil },
		"operator raft list-peers":      func() (cli.Command, error) { return operraftlist.New(ui), nil },
		"operator raft remove-peer":     func() (cli.Command, error) { return operraftremove.New(ui), nil },
		"reload":                        func() (cli.Command, error) { return reload.New(ui), nil },
		"rtt":                           func() (cli.Command, error) { return rtt.New(ui), nil },
		"snapshot":                      func() (cli.Command, error) { return snapshot.New(), nil },
		"snapshot inspect":              func() (cli.Command, error) { return snapshotinspect.New(ui), nil },
		"snapshot restore":              func() (cli.Command, error) { return snapshotrestore.New(ui), nil },
		"snapshot save":                 func() (cli.Command, error) { return snapshotsave.New(ui), nil },
		"validate":                      func() (cli.Command, error) { return validate.New(ui), nil },
		"version":                       func() (cli.Command, error) { return versioncmd.New(ui, version.GetHumanVersion()), nil },
		"watch":                         func() (cli.Command, error) { return watchcmd.New(ui, makeShutdownCh()), nil },
	}
}

// makeShutdownCh returns a channel that can be used for shutdown
// notifications for commands. This channel will send a message for every
// interrupt or SIGTERM received.
func makeShutdownCh() <-chan struct{} {
	resultCh := make(chan struct{})

	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		for {
			<-signalCh
			resultCh <- struct{}{}
		}
	}()

	return resultCh
}
