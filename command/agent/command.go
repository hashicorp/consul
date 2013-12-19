package agent

import (
	"github.com/hashicorp/logutils"
	"github.com/mitchellh/cli"
	"strings"
	"time"
)

// gracefulTimeout controls how long we wait before forcefully terminating
var gracefulTimeout = 3 * time.Second

// Command is a Command implementation that runs a Serf agent.
// The command will not end unless a shutdown message is sent on the
// ShutdownCh. If two messages are sent on the ShutdownCh it will forcibly
// exit.
type Command struct {
	Ui         cli.Ui
	ShutdownCh <-chan struct{}
	args       []string
	logFilter  *logutils.LevelFilter
}

func (c *Command) Run(args []string) int {
	return 0
}

func (c *Command) Synopsis() string {
	return "Runs a Consul agent"
}

func (c *Command) Help() string {
	helpText := `
Usage: consul agent [options]

  Starts the Consul agent and runs until an interrupt is received. The
  agent represents a single node in a cluster.

Options:

  -bind=0.0.0.0            Address to bind network listeners to
  -config-file=foo         Path to a JSON file to read configuration from.
                           This can be specified multiple times.
  -config-dir=foo          Path to a directory to read configuration files
                           from. This will read every file ending in ".json"
                           as configuration in this directory in alphabetical
                           order.
  -encrypt=foo             Key for encrypting network traffic within Serf.
                           Must be a base64-encoded 16-byte key.
  -event-handler=foo       Script to execute when events occur. This can
                           be specified multiple times. See the event scripts
                           section below for more info.
  -join=addr               An initial agent to join with. This flag can be
                           specified multiple times.
  -log-level=info          Log level of the agent.
  -node=hostname           Name of this node. Must be unique in the cluster
  -profile=[lan|wan|local] Profile is used to control the timing profiles used in Serf.
						   The default if not provided is lan.
  -protocol=n              Serf protocol version to use. This defaults to
                           the latest version, but can be set back for upgrades.
  -role=foo                The role of this node, if any. This can be used
                           by event scripts to differentiate different types
                           of nodes that may be part of the same cluster.
  -rpc-addr=127.0.0.1:7373 Address to bind the RPC listener.
  -snapshot=path/to/file   The snapshot file is used to store alive nodes and
                           event information so that Serf can rejoin a cluster
						   and avoid event replay on restart.

Event handlers:

  For more information on what event handlers are, please read the
  Serf documentation. This section will document how to configure them
  on the command-line. There are three methods of specifying an event
  handler:

  - The value can be a plain script, such as "event.sh". In this case,
    Serf will send all events to this script, and you'll be responsible
    for differentiating between them based on the SERF_EVENT.

  - The value can be in the format of "TYPE=SCRIPT", such as
    "member-join=join.sh". With this format, Serf will only send events
    of that type to that script.

  - The value can be in the format of "user:EVENT=SCRIPT", such as
    "user:deploy=deploy.sh". This means that Serf will only invoke this
    script in the case of user events named "deploy".
`
	return strings.TrimSpace(helpText)
}
