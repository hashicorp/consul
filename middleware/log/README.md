# log

*log* enables query logging.

## Syntax

~~~ txt
log
~~~

* With no arguments, a query log entry is written to query.log in the common log format for all requests

~~~ txt
log FILE
~~~

* **FILE** is the log file to create (or append to).

~~~ txt
log [NAME] FILE [FORMAT]
~~~

* `NAME` is the name to match in order to be logged
* `FILE` is the log file to create (or append to)
* `FORMAT` is the log format to use (default is Common Log Format)

You can further specify the class of responses that get logged:

~~~ txt
log [NAME] FILE [FORMAT] {
    class [success|denial|error|all]
}
~~~

Here `success` `denial` and `error` denotes the class of responses that should be logged. The
classes have the following meaning:

* `success`: successful response
* `denial`: either NXDOMAIN or NODATA (name exists, type does not)
* `error`: SERVFAIL, NOTIMP, REFUSED, etc. Anything that indicates the remote server is not willing to
    resolve the request.
* `all`: the default - nothing is specified.

If no class is specified, it defaults to *all*.

## Log File

The log file can be any filename. It could also be *stdout* or *stderr* to write the log to the console,
or *syslog* to write to the system log (except on Windows). If the log file does not exist beforehand,
CoreDNS will create it before appending to it.

## Log Format

You can specify a custom log format with any placeholder values. Log supports both request and
response placeholders.

The following place holders are supported:

* `{type}`: qtype of the request.
* `{name}`: qname of the request.
* `{class}`: qclass of the request.
* `{proto}`: protocol used (tcp or udp).
* `{when}`: time of the query.
* `{remote}`: client's IP address.
* `{port}`: client's port.
* `{rcode}`: response RCODE.
* `{size}`: response size.
* `{duration}`: response duration.
* `{>bufsize}`: the EDNS0 buffer size advertized by the client.
* `{>do}`: is the EDNS0 DO (DNSSEC OK) bit set.
* `{>id}`: query ID
* `{>opcode}`: query OPCODE

The default Common Log Format is:

~~~ txt
`{remote} - [{when}] "{type} {class} {name} {proto} {>do} {>bufsize}" {rcode} {size} {duration}`
~~~

## Examples

Log all requests to a file:

~~~
log /var/log/query.log
~~~

Custom log format:

~~~
log . ../query.log "{proto} Request: {name} {type} {>id}"
~~~

Only log denials for example.org (and below to a file)

~~~
log example.org example-query-log {
    class denial
}
~~~
