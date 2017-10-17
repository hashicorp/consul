package command

import (
	"os"
	"os/signal"
	"syscall"

	agentcmd "github.com/hashicorp/consul/command/agent"
	"github.com/hashicorp/consul/command/catalog"
	catlistdccmd "github.com/hashicorp/consul/command/catalog/list/dc"
	catlistnodescmd "github.com/hashicorp/consul/command/catalog/list/nodes"
	catlistsvccmd "github.com/hashicorp/consul/command/catalog/list/services"
	"github.com/hashicorp/consul/command/event"
	execmd "github.com/hashicorp/consul/command/exec"
	"github.com/hashicorp/consul/command/forceleave"
	"github.com/hashicorp/consul/command/info"
	"github.com/hashicorp/consul/command/join"
	"github.com/hashicorp/consul/command/keygen"
	"github.com/hashicorp/consul/command/keyring"
	"github.com/hashicorp/consul/command/kv"
	kvdel "github.com/hashicorp/consul/command/kv/del"
	kvexp "github.com/hashicorp/consul/command/kv/exp"
	kvget "github.com/hashicorp/consul/command/kv/get"
	kvimp "github.com/hashicorp/consul/command/kv/imp"
	kvput "github.com/hashicorp/consul/command/kv/put"
	"github.com/hashicorp/consul/command/leave"
	"github.com/hashicorp/consul/command/lock"
	"github.com/hashicorp/consul/command/maint"
	"github.com/hashicorp/consul/command/members"
	"github.com/hashicorp/consul/command/monitor"
	"github.com/hashicorp/consul/command/operator"
	operauto "github.com/hashicorp/consul/command/operator/autopilot"
	operautoget "github.com/hashicorp/consul/command/operator/autopilot/get"
	operautoset "github.com/hashicorp/consul/command/operator/autopilot/set"
	operraft "github.com/hashicorp/consul/command/operator/raft"
	operraftlist "github.com/hashicorp/consul/command/operator/raft/listpeers"
	operraftremove "github.com/hashicorp/consul/command/operator/raft/removepeer"
	"github.com/hashicorp/consul/command/reload"
	"github.com/hashicorp/consul/command/rtt"
	"github.com/hashicorp/consul/command/snapshot"
	snapinspectcmd "github.com/hashicorp/consul/command/snapshot/inspect"
	snaprestorecmd "github.com/hashicorp/consul/command/snapshot/restore"
	snapsavecmd "github.com/hashicorp/consul/command/snapshot/save"
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

		"catalog":                       func() (cli.Command, error) { return catalog.New(), nil },
		"catalog datacenters":           func() (cli.Command, error) { return catlistdccmd.New(ui), nil },
		"catalog nodes":                 func() (cli.Command, error) { return catlistnodescmd.New(ui), nil },
		"catalog services":              func() (cli.Command, error) { return catlistsvccmd.New(ui), nil },
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
		"operator":                      func() (cli.Command, error) { return operator.New(), nil },
		"operator autopilot":            func() (cli.Command, error) { return operauto.New(), nil },
		"operator autopilot get-config": func() (cli.Command, error) { return operautoget.New(ui), nil },
		"operator autopilot set-config": func() (cli.Command, error) { return operautoset.New(ui), nil },
		"operator raft":                 func() (cli.Command, error) { return operraft.New(), nil },
		"operator raft list-peers":      func() (cli.Command, error) { return operraftlist.New(ui), nil },
		"operator raft remove-peer":     func() (cli.Command, error) { return operraftremove.New(ui), nil },
		"reload":                        func() (cli.Command, error) { return reload.New(ui), nil },
		"rtt":                           func() (cli.Command, error) { return rtt.New(ui), nil },
		"snapshot":                      func() (cli.Command, error) { return snapshot.New(), nil },
		"snapshot inspect":              func() (cli.Command, error) { return snapinspectcmd.New(ui), nil },
		"snapshot restore":              func() (cli.Command, error) { return snaprestorecmd.New(ui), nil },
		"snapshot save":                 func() (cli.Command, error) { return snapsavecmd.New(ui), nil },
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
