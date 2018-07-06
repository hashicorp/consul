# reload

## Name

*reload* - allows automatic reload of a changed Corefile.

## Description

This plugin allows automatic reload of a changed _Corefile_.
To enable automatic reloading of _zone file_ changes, use the `auto` plugin.

This plugin periodically checks if the Corefile has changed by reading
it and calculating its MD5 checksum. If the file has changed, it reloads
CoreDNS with the new Corefile. This eliminates the need to send a SIGHUP
or SIGUSR1 after changing the Corefile.

The reloads are graceful - you should not see any loss of service when the
reload happens. Even if the new Corefile has an error, CoreDNS will continue
to run the old config and an error message will be printed to the log. But see
the Bugs section for failure modes.

In some environments (for example, Kubernetes), there may be many CoreDNS
instances that started very near the same time and all share a common
Corefile. To prevent these all from reloading at the same time, some
jitter is added to the reload check interval. This is jitter from the
perspective of multiple CoreDNS instances; each instance still checks on a
regular interval, but all of these instances will have their reloads spread
out across the jitter duration. This isn't strictly necessary given that the
reloads are graceful, and can be disabled by setting the jitter to `0s`.

Jitter is re-calculated whenever the Corefile is reloaded.

This plugin can only be used once per Server Block.

## Syntax

~~~ txt
reload [INTERVAL] [JITTER]
~~~

* The plugin will check for changes every **INTERVAL**, subject to +/- the **JITTER** duration
* **INTERVAL** and **JITTER** are Golang (durations)[https://golang.org/pkg/time/#ParseDuration]
* Default **INTERVAL** is 30s, default **JITTER** is 15s
* Minimal value for **INTERVAL** is 2s, and for **JITTER** is 1s
* If **JITTER** is more than half of **INTERVAL**, it will be set to half of **INTERVAL**

## Examples

Check with the default intervals:

~~~ corefile
. {
    reload
    erratic
}
~~~

Check every 10 seconds (jitter is automatically set to 10 / 2 = 5 in this case):

~~~ corefile
. {
    reload 10s
    erratic
}
~~~

## Bugs

The reload happens without data loss (i.e. DNS queries keep flowing), but there is a corner case
where the reload fails, and you loose functionality. Consider the following Corefile:

~~~ txt
. {
	health :8080
	whoami
}
~~~

CoreDNS starts and serves health from :8080. Now you change `:8080` to `:443` not knowing a process
is already listening on that port. The process reloads and performs the following steps:

1. close the listener on 8080
2. reload and parse the config again
3. fail to start a new listener on 443
4. fail loading the new Corefile, abort and keep using the old process

After the aborted attempt to reload we are left with the old proceses running, but the listener is
closed in step 1; so the health endpoint is broken. The same can hopen in the prometheus metrics plugin.

In general be careful with assigning new port and expecting reload to work fully.
