# log

`log` enables request logging. The request log is also known from some vernaculars as an access log.

## Syntax

~~~
log
~~~

* With no arguments, an query log is written to query.log in the common log format for all requests
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
* `{class}`: class of the request.
* `{proto}`: protocol used (tcp or udp).
* `{when}`: time of the query.
* `{remote}`: client's IP address.
* `{port}`: client's port.
* `{rcode}`: response RCODE.
* `{size}`: response size.
* `{duration}`: response duration (in seconds).
* `{>bufsize}`: the EDNS0 buffer size advertized by the client.
* `{>do}`: is the EDNS0 DO (DNSSEC OK) bit set.
* `{>id}`: query ID
* `{>opcode}`: query OPCODE


## Log Rotation

If you enable log rotation, log files will be automatically maintained when they get large or old.
You can use rotation by opening a block on your first line, which can be any of the variations
described above:

~~~
log ... {
    rotate {
    size maxsize
    age  maxage
    keep maxkeep
    }
}
~~~

* `maxsize` is the maximum size of a log file in megabytes (MB) before it gets rotated. Default is 100 MB.
* `maxage` is the maximum age of a rotated log file in days, after which it will be deleted. Default is to never delete old files because of age.
* `maxkeep` is the maximum number of rotated log files to keep. Default is to retain all old log files.

## Examples

Log all requests to a file:

~~~
log /var/log/query.log
~~~

Custom log format:

~~~
log . ../query.log "{proto} Request: {name} {type} {>id}"
~~~

With rotation:

~~~
log query.log {
    rotate {
        100 # Rotate after 100 MB
        age  14  # Keep log files for 14 days
        keep 10  # Keep at most 10 log files
    }
}
