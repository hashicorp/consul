## Name

*coredns* - plugable DNS nameserver optimized for service discovery.

## Synopsis

*coredns* *[OPTION]*...

## Description

CoreDNS is a DNS server that chains plugins. Each plugin handles a DNS feature, like rewriting
queries, kubernetes service discovery or just exporting metrics. There are many other plugins,
each described on <https://coredns.io/plugins> and there respective manual pages.

When started with no options CoreDNS will looks for a file names `Corefile` in the current
directory, if found it will parse its contents and start up accordingly. If no `Corefile` is found
it will start with the *whoami* plugin (coredns-whoami(7)) and start listening on port 53 (unless
overriden with `-dns.port`).

Available options:

**-conf** **FILE**
: specificy Corefile to load.

**-cpu** **CAP**
: specify maximum CPU capacity in percent.

**-dns.port** **PORT**
: override default port to listen on.

**-pidfile** **FILE**
: write PID to **FILE**.

**-plugins**
: list all plugins and quit.

**-quiet**
: don't print any version and port information on startup.

**-version**
: show version and quit.

## Authors

CoreDNS Authors.

## Copyright

Apache License 2.0

## See Also

Corefile(5) @@PLUGINS@@.
