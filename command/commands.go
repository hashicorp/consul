package command

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/hashicorp/consul/command/event"
	execmd "github.com/hashicorp/consul/command/exec"
	"github.com/hashicorp/consul/command/info"
	"github.com/hashicorp/consul/command/join"
	"github.com/hashicorp/consul/command/keygen"
	"github.com/hashicorp/consul/command/kv"
	"github.com/hashicorp/consul/command/kvdel"
	"github.com/hashicorp/consul/command/kvexp"
	"github.com/hashicorp/consul/command/kvimp"
	"github.com/hashicorp/consul/command/validate"
	"github.com/hashicorp/consul/version"
	"github.com/mitchellh/cli"
)

// Commands is the mapping of all the available Consul commands.
var Commands map[string]cli.CommandFactory

func init() {
	ui := &cli.BasicUi{Writer: os.Stdout, ErrorWriter: os.Stderr}

	Commands = map[string]cli.CommandFactory{
		"agent": func() (cli.Command, error) {
			return &AgentCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetNone,
					UI:    ui,
				},
				Revision:          version.GitCommit,
				Version:           version.Version,
				VersionPrerelease: version.VersionPrerelease,
				HumanVersion:      version.GetHumanVersion(),
				ShutdownCh:        make(chan struct{}),
			}, nil
		},

		"catalog": func() (cli.Command, error) {
			return &CatalogCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetNone,
					UI:    ui,
				},
			}, nil
		},

		"catalog datacenters": func() (cli.Command, error) {
			return &CatalogListDatacentersCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
		},

		"catalog nodes": func() (cli.Command, error) {
			return &CatalogListNodesCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
		},

		"catalog services": func() (cli.Command, error) {
			return &CatalogListServicesCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
		},

		"event": func() (cli.Command, error) {
			return event.New(ui), nil
		},

		"exec": func() (cli.Command, error) {
			return execmd.New(ui, makeShutdownCh()), nil
		},

		"force-leave": func() (cli.Command, error) {
			return &ForceLeaveCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetClientHTTP,
					UI:    ui,
				},
			}, nil
		},

		"info": func() (cli.Command, error) {
			return info.New(ui), nil
		},

		"join": func() (cli.Command, error) {
			return join.New(ui), nil
		},

		"keygen": func() (cli.Command, error) {
			return keygen.New(ui), nil
		},

		"keyring": func() (cli.Command, error) {
			return &KeyringCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetClientHTTP,
					UI:    ui,
				},
			}, nil
		},

		"kv": func() (cli.Command, error) {
			return kv.New(), nil
		},

		"kv delete": func() (cli.Command, error) {
			return kvdel.New(ui), nil
		},

		"kv get": func() (cli.Command, error) {
			return &KVGetCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
		},

		"kv put": func() (cli.Command, error) {
			return &KVPutCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
		},

		"kv export": func() (cli.Command, error) {
			return kvexp.New(ui), nil
		},

		"kv import": func() (cli.Command, error) {
			return kvimp.New(ui), nil
		},

		"leave": func() (cli.Command, error) {
			return &LeaveCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetClientHTTP,
					UI:    ui,
				},
			}, nil
		},

		"lock": func() (cli.Command, error) {
			return &LockCommand{
				ShutdownCh: makeShutdownCh(),
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
		},

		"maint": func() (cli.Command, error) {
			return &MaintCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetClientHTTP,
					UI:    ui,
				},
			}, nil
		},

		"members": func() (cli.Command, error) {
			return &MembersCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetClientHTTP,
					UI:    ui,
				},
			}, nil
		},

		"monitor": func() (cli.Command, error) {
			return &MonitorCommand{
				ShutdownCh: makeShutdownCh(),
				BaseCommand: BaseCommand{
					Flags: FlagSetClientHTTP,
					UI:    ui,
				},
			}, nil
		},

		"operator": func() (cli.Command, error) {
			return &OperatorCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetNone,
					UI:    ui,
				},
			}, nil
		},

		"operator autopilot": func() (cli.Command, error) {
			return &OperatorAutopilotCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetNone,
					UI:    ui,
				},
			}, nil
		},

		"operator autopilot get-config": func() (cli.Command, error) {
			return &OperatorAutopilotGetCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
		},

		"operator autopilot set-config": func() (cli.Command, error) {
			return &OperatorAutopilotSetCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
		},

		"operator raft": func() (cli.Command, error) {
			return &OperatorRaftCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
		},

		"operator raft list-peers": func() (cli.Command, error) {
			return &OperatorRaftListCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
		},

		"operator raft remove-peer": func() (cli.Command, error) {
			return &OperatorRaftRemoveCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
		},

		"reload": func() (cli.Command, error) {
			return &ReloadCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetClientHTTP,
					UI:    ui,
				},
			}, nil
		},

		"rtt": func() (cli.Command, error) {
			return &RTTCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetClientHTTP,
					UI:    ui,
				},
			}, nil
		},

		"snapshot": func() (cli.Command, error) {
			return &SnapshotCommand{
				UI: ui,
			}, nil
		},

		"snapshot restore": func() (cli.Command, error) {
			return &SnapshotRestoreCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
		},

		"snapshot save": func() (cli.Command, error) {
			return &SnapshotSaveCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
		},

		"snapshot inspect": func() (cli.Command, error) {
			return &SnapshotInspectCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetNone,
					UI:    ui,
				},
			}, nil
		},

		"validate": func() (cli.Command, error) {
			return validate.New(ui), nil
		},

		"version": func() (cli.Command, error) {
			return &VersionCommand{
				HumanVersion: version.GetHumanVersion(),
				UI:           ui,
			}, nil
		},

		"watch": func() (cli.Command, error) {
			return &WatchCommand{
				ShutdownCh: makeShutdownCh(),
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
		},
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

// mapToKV converts a map[string]string into a human-friendly key=value list,
// sorted by name.
func mapToKV(m map[string]string, joiner string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	r := make([]string, len(keys))
	for i, k := range keys {
		r[i] = fmt.Sprintf("%s=%s", k, m[k])
	}
	return strings.Join(r, joiner)
}
