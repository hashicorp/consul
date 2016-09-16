# log

`log` enables request logging. The request log is also known in some vernacular as an access log.

## Syntax

~~~
log
~~~

* With no arguments, a query log entry is written to query.log in the common log format for all requests
    (base name = .).

~~~
log file
~~~

* file is the log file to create (or append to). The base path is assumed to be . .

~~~
log name file [format]
~~~

* `name` is the base name to match in order to be logged
* `file` is the log file to create (or append to)
* `format` is the log format to use (default is Common Log Format)

## Log File

The log file can be any filename. It could also be stdout or stderr to write the log to the console,
or syslog to write to the system log (except on Windows). If the log file does not exist beforehand,
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


## Examples

Log all requests to a file:

~~~
log /var/log/query.log
~~~

Custom log format:

~~~
log . ../query.log "{proto} Request: {name} {type} {>id}"
~~~
