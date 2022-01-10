# Command-Line Interface (CLI)

This section is a work in progress.

The `consul` binary provides a CLI for interacting with the [HTTP API]. Some commands may
also exec other processes or generate data used by Consul (ex: tls certificates). The
`agent` command is responsible for starting the Consul agent.

The [cli reference] in Consul user documentation has a full reference to all available
commands.

[HTTP API]: ../http-api
[cli reference]: https://www.consul.io/commands

## Code

The CLI entrypoint is [main.go] and the majority of the source for the CLI is under the
[command] directory. Each subcommand is a separate package under [command]. The CLI uses
[github.com/mitchellh/cli] as a framework, and uses the [flag] package from the stdlib for
command line flags.


[command]: https://github.com/hashicorp/consul/tree/main/command
[main.go]: https://github.com/hashicorp/consul/blob/main/main.go
[flag]: https://pkg.go.dev/flag
[github.com/mitchellh/cli]: https://github.com/mitchellh/cli

## Important notes

The [cli.Ui] wraps an `io.Writer` for both stdout and stderr. At the time of writing both
`Info` and `Output` go to stdout. Writing `Info` to stdout has been a source of a couple
bugs. To prevent these bugs in the future it is recommended that `Info` should no longer
be used. Instead, send all information messages to stderr by using `Warn`.


[cli.Ui]: https://pkg.go.dev/github.com/mitchellh/cli#Ui
