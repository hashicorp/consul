package command

import (
	"os"
	"os/signal"
	"syscall"

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

		"configtest": func() (cli.Command, error) {
			return &ConfigTestCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetNone,
					UI:    ui,
				},
			}, nil
		},

		"event": func() (cli.Command, error) {
			return &EventCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
		},

		"exec": func() (cli.Command, error) {
			return &ExecCommand{
				ShutdownCh: makeShutdownCh(),
				BaseCommand: BaseCommand{
					Flags: FlagSetHTTP,
					UI:    ui,
				},
			}, nil
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
			return &InfoCommand{
				BaseCommand: BaseCommand{
					UI:    ui,
					Flags: FlagSetClientHTTP,
				},
			}, nil
		},

		"join": func() (cli.Command, error) {
			return &JoinCommand{
				BaseCommand: BaseCommand{
					UI:    ui,
					Flags: FlagSetClientHTTP,
				},
			}, nil
		},

		"keygen": func() (cli.Command, error) {
			return &KeygenCommand{
				BaseCommand: BaseCommand{
					UI:    ui,
					Flags: FlagSetNone,
				},
			}, nil
		},

		"keyring": func() (cli.Command, error) {
			return &KeyringCommand{
				BaseCommand: BaseCommand{
					UI:    ui,
					Flags: FlagSetClientHTTP,
				},
			}, nil
		},

		"kv": func() (cli.Command, error) {
			return &KVCommand{
				BaseCommand: BaseCommand{
					UI:    ui,
					Flags: FlagSetNone,
				},
			}, nil
		},

		"kv delete": func() (cli.Command, error) {
			return &KVDeleteCommand{
				BaseCommand: BaseCommand{
					UI:    ui,
					Flags: FlagSetHTTP,
				},
			}, nil
		},

		"kv get": func() (cli.Command, error) {
			return &KVGetCommand{
				BaseCommand: BaseCommand{
					UI:    ui,
					Flags: FlagSetHTTP,
				},
			}, nil
		},

		"kv put": func() (cli.Command, error) {
			return &KVPutCommand{
				BaseCommand: BaseCommand{
					UI:    ui,
					Flags: FlagSetHTTP,
				},
			}, nil
		},

		"kv export": func() (cli.Command, error) {
			return &KVExportCommand{
				BaseCommand: BaseCommand{
					UI:    ui,
					Flags: FlagSetHTTP,
				},
			}, nil
		},

		"kv import": func() (cli.Command, error) {
			return &KVImportCommand{
				BaseCommand: BaseCommand{
					UI:    ui,
					Flags: FlagSetHTTP,
				},
			}, nil
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
			return &ValidateCommand{
				BaseCommand: BaseCommand{
					Flags: FlagSetNone,
					UI:    ui,
				},
			}, nil
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
