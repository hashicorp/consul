## CoreDNS

*coredns* - plugable DNS nameserver optimized for service discovery and flexibility.

## Synopsis

*coredns* **[-conf FILE]** **[-dns.port PORT}** **[OPTION]**... 

## Description

CoreDNS is a DNS server that chains plugins. Each plugin handles a DNS feature, like rewriting
queries, kubernetes service discovery or just exporting metrics. There are many other plugins,
each described on <https://coredns.io/plugins> and their respective manual pages. Plugins not
bundled by default in CoreDNS are listed on <https://coredns.io/explugins>.

When started without options CoreDNS will look for a file named `Corefile` in the current
directory, if found, it will parse its contents and start up accordingly. If no `Corefile` is found
it will start with the *whoami* plugin (coredns-whoami(7)) and start listening on port 53 (unless
overridden with `-dns.port`).

Available options:

**-conf** **FILE**
: specify Corefile to load, if not given CoreDNS will look for a `Corefile` in the current
  directory.

**-dns.port** **PORT**
: override default port (53) to listen on.

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
