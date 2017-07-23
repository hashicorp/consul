# hosts

*hosts* enables serving zone data from a /etc/hosts style file.

The hosts middleware is useful for serving zones from a /etc/hosts file. It serves from a preloaded
file that exists on disk. It checks the file for changes and updates the zones accordingly. This
middleware only supports A, AAAA, and PTR records. The hosts middleware can be used with readily
available hosts files that block access to advertising servers.

## Syntax

~~~
hosts [FILE [ZONES...]] {
    fallthrough
}
~~~

* **FILE** the hosts file to read and parse. If the path is relative the path from the *root*
  directive will be prepended to it. Defaults to /etc/hosts if omitted
* **ZONES** zones it should be authoritative for. If empty, the zones from the configuration block
    are used.
* `fallthrough` If zone matches and no record can be generated, pass request to the next middleware.

## Examples

Load `/etc/hosts` file.

~~~
hosts
~~~

Load `example.hosts` file in the current directory.

~~~
hosts example.hosts
~~~

Load example.hosts file and only serve example.org and example.net from it and fall through to the
next middleware if query doesn't match.

~~~
hosts example.hosts example.org example.net {
    fallthrough
}
~~~
