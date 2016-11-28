# go-sockaddr

## `sockaddr` Library

Socket address convenience functions for Go.  `go-sockaddr` is a convenience
library that makes doing the right thing with IP addresses easy.  `go-sockaddr`
is loosely modeled after the UNIX `sockaddr_t` and creates a union of the family
of `sockaddr_t` types (see below for an ascii diagram).  Library documentation
is available
at
[https://godoc.org/github.com/hashicorp/go-sockaddr](https://godoc.org/github.com/hashicorp/go-sockaddr).
The primary intent of the library was to make it possible to define heuristics
for selecting IP addresses at process initialization time.  See
the
[docs](https://godoc.org/github.com/hashicorp/go-sockaddr),
[`template` package](https://godoc.org/github.com/hashicorp/go-sockaddr/template),
tests,
and
[CLI utility](https://github.com/hashicorp/go-sockaddr/tree/master/cmd/sockaddr)
for details and hints as to how to use this library.

With this library it is possible to find an IP address that:

* is attached to a default route
  ([`GetDefaultInterfaces()`](https://godoc.org/github.com/hashicorp/go-sockaddr#GetDefaultInterfaces))
* is an RFC1918 address
  ([`IfByRFC("1918")`](https://godoc.org/github.com/hashicorp/go-sockaddr#IfByRFC))
* ordered
  ([`OrderedIfAddrBy(args)`](https://godoc.org/github.com/hashicorp/go-sockaddr#OrderedIfAddrBy) where
  `args` includes, but is not limited
  to,
  [`AscIfType`](https://godoc.org/github.com/hashicorp/go-sockaddr#AscIfType),
  [`AscNetworkSize`](https://godoc.org/github.com/hashicorp/go-sockaddr#AscNetworkSize))
* excludes all IPv6 addresses
  ([`IfByType("^(IPv4)$")`](https://godoc.org/github.com/hashicorp/go-sockaddr#IfByType)); and
* is larger than a `/32`
  ([`IfByMaskSize(32)`](https://godoc.org/github.com/hashicorp/go-sockaddr#IfByMaskSize))

or:

* exclude all interfaces that are `down`
  ([`ExcludeIfs("flags", "down")`](https://godoc.org/github.com/hashicorp/go-sockaddr#ExcludeIfs))
* preferring IPv6 over IPv4
  ([`SortIfByType()`](https://godoc.org/github.com/hashicorp/go-sockaddr#SortIfByType) +
  [`ReverseIfAddrs()`](https://godoc.org/github.com/hashicorp/go-sockaddr#ReverseIfAddrs))
* and excluding any IP in RFC6890 address
  ([`IfByRFC("6890")`](https://godoc.org/github.com/hashicorp/go-sockaddr#IfByRFC))

There are also a few simple helper functions such as `GetPublicIP` and
`GetPrivateIP` which both return strings and select the first public or private
IP address on the default interface, respectively.

## `sockaddr` CLI

Given the possible complexity of the `sockaddr` library, there is a CLI utility
that accompanies the library, also
called
[`sockaddr`](https://github.com/hashicorp/go-sockaddr/tree/master/cmd/sockaddr).
The
[`sockaddr`](https://github.com/hashicorp/go-sockaddr/tree/master/cmd/sockaddr)
utility exposes nearly all of the functionailty of the library and can be used
either as an administrative tool or testing tool.  To install
the
[`sockaddr`](https://github.com/hashicorp/go-sockaddr/tree/master/cmd/sockaddr),
run:

```text
$ go install github.com/hashicorp/go-sockaddr/cmd/sockaddr
```

If you're familiar with UNIX's `sockaddr` struct's, the following diagram
mapping the C `sockaddr` (top) to `go-sockaddr` structs (bottom) and
interfaces will be helpful:

```
+-------------------------------------------------------+
|                                                       |
|                        sockaddr                       |
|                        SockAddr                       |
|                                                       |
| +--------------+ +----------------------------------+ |
| | sockaddr_un  | |                                  | |
| | SockAddrUnix | |           sockaddr_in{,6}        | |
| +--------------+ |                IPAddr            | |
|                  |                                  | |
|                  | +-------------+ +--------------+ | |
|                  | | sockaddr_in | | sockaddr_in6 | | |
|                  | |   IPv4Addr  | |   IPv6Addr   | | |
|                  | +-------------+ +--------------+ | |
|                  |                                  | |
|                  +----------------------------------+ |
|                                                       |
+-------------------------------------------------------+
```
