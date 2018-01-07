# hosts

## Name

*hosts* - enables serving zone data from a `/etc/hosts` style file.

## Description

The hosts plugin is useful for serving zones from a /etc/hosts file. It serves from a preloaded
file that exists on disk. It checks the file for changes and updates the zones accordingly. This
plugin only supports A, AAAA, and PTR records. The hosts plugin can be used with readily
available hosts files that block access to advertising servers.

## Syntax

~~~
hosts [FILE [ZONES...]] {
    [INLINE]
    fallthrough [ZONES...]
}
~~~

* **FILE** the hosts file to read and parse. If the path is relative the path from the *root*
  directive will be prepended to it. Defaults to /etc/hosts if omitted. We scan the file for changes
  every 5 seconds.
* **ZONES** zones it should be authoritative for. If empty, the zones from the configuration block
   are used.
* **INLINE** the hosts file contents inlined in Corefile. If there are any lines before fallthrough
   then all of them will be treated as the additional content for hosts file. The specified hosts
   file path will still be read but entries will be overrided.
* `fallthrough` If zone matches and no record can be generated, pass request to the next plugin.
  If **[ZONES...]** is omitted, then fallthrough happens for all zones for which the plugin
  is authoritative. If specific zones are listed (for example `in-addr.arpa` and `ip6.arpa`), then only
  queries for those zones will be subject to fallthrough.

## Examples

Load `/etc/hosts` file.

~~~ corefile
. {
    hosts
}
~~~

Load `example.hosts` file in the current directory.

~~~
hosts example.hosts
~~~

Load example.hosts file and only serve example.org and example.net from it and fall through to the
next plugin if query doesn't match.

~~~
hosts example.hosts example.org example.net {
    fallthrough
}
~~~

Load hosts file inlined in Corefile.

~~~
hosts example.hosts example.org {
    10.0.0.1 example.org
    fallthrough
}
~~~
