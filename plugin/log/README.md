# log

## Name

*log* - enables query logging to standard output.

## Description

By just using *log* you dump all queries (and parts for the reply) on standard output. Options exist
to tweak the output a little. The date/time prefix on log lines is RFC3339 formatted with
milliseconds.

Note that for busy servers logging will incur a performance hit.

## Syntax

~~~ txt
log
~~~

* With no arguments, a query log entry is written to *stdout* in the common log format for all requests

Or if you want/need slightly more control:

~~~ txt
log [NAMES...] [FORMAT]
~~~

* `NAMES` is the name list to match in order to be logged
* `FORMAT` is the log format to use (default is Common Log Format), `{common}` is used as a shortcut
  for the Common Log Format. You can also use `{combined}` for a format that adds the query opcode
  `{>opcode}` to the Common Log Format.

You can further specify the classes of responses that get logged:

~~~ txt
log [NAMES...] [FORMAT] {
    class CLASSES...
}
~~~

* `CLASSES` is a space-separated list of classes of responses that should be logged

The classes of responses have the following meaning:

* `success`: successful response
* `denial`: either NXDOMAIN or nodata responses (Name exists, type does not). A nodata response
   sets the return code to NOERROR.
* `error`: SERVFAIL, NOTIMP, REFUSED, etc. Anything that indicates the remote server is not willing to
    resolve the request.
* `all`: the default - nothing is specified. Using of this class means that all messages will be
  logged whatever we mix together with "all".

If no class is specified, it defaults to *all*.

## Log Format

You can specify a custom log format with any placeholder values. Log supports both request and
response placeholders.

The following place holders are supported:

* `{type}`: qtype of the request
* `{name}`: qname of the request
* `{class}`: qclass of the request
* `{proto}`: protocol used (tcp or udp)
* `{remote}`: client's IP address, for IPv6 addresses these are enclosed in brackets: `[::1]`
* `{local}`: server's IP address, for IPv6 addresses these are enclosed in brackets: `[::1]`
* `{size}`: request size in bytes
* `{port}`: client's port
* `{duration}`: response duration
* `{rcode}`: response RCODE
* `{rsize}`: raw (uncompressed), response size (a client may receive a smaller response)
* `{>rflags}`: response flags, each set flag will be displayed, e.g. "aa, tc". This includes the qr
  bit as well
* `{>bufsize}`: the EDNS0 buffer size advertised in the query
* `{>do}`: is the EDNS0 DO (DNSSEC OK) bit set in the query
* `{>id}`: query ID
* `{>opcode}`: query OPCODE
* `{common}`: the default Common Log Format.
* `{combined}`: the Common Log Format with the query opcode.
* `{/LABEL}`: any metadata label is accepted as a place holder if it is enclosed between `{/` and
  `}`, the place holder will be replaced by the corresponding metadata value or the default value
  `-` if label is not defined. See the *metadata* plugin for more information.

The default Common Log Format is:

~~~ txt
`{remote}:{port} - {>id} "{type} {class} {name} {proto} {size} {>do} {>bufsize}" {rcode} {>rflags} {rsize} {duration}`
~~~

Each of these logs will be outputted with `log.Infof`, so a typical example looks like this:

~~~ txt
[INFO] [::1]:50759 - 29008 "A IN example.org. udp 41 false 4096" NOERROR qr,rd,ra,ad 68 0.037990251s
~~~~

## Examples

Log all requests to stdout

~~~ corefile
. {
    log
    whoami
}
~~~

Custom log format, for all zones (`.`)

~~~ corefile
. {
    log . "{proto} Request: {name} {type} {>id}"
}
~~~

Only log denials (NXDOMAIN and nodata) for example.org (and below)

~~~ corefile
. {
    log example.org {
        class denial
    }
}
~~~

Log all queries which were not resolved successfully in the Combined Log Format.

~~~ corefile
. {
    log . {combined} {
        class denial error
    }
}
~~~

Log all queries on which we did not get errors

~~~ corefile
. {
    log . {
        class denial success
    }
}
~~~

Also the multiple statements can be OR-ed, for example, we can rewrite the above case as following:

~~~ corefile
. {
    log . {
        class denial
        class success
    }
}
~~~
