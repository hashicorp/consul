# reload

## Name

*reload* - allows automatic reload of a changed Corefile

## Description

This plugin periodically checks if the Corefile has changed by reading
it and calculating its MD5 checksum. If the file has changed, it reloads
CoreDNS with the new Corefile. This eliminates the need to send a SIGHUP
or SIGUSR1 after changing the Corefile.

The reloads are graceful - you should not see any loss of service when the
reload happens. Even if the new Corefile has an error, CoreDNS will continue
to run the old config and an error message will be printed to the log.

In some environments (for example, Kubernetes), there may be many CoreDNS 
instances that started very near the same time and all share a common
Corefile. To prevent these all from reloading at the same time, some
jitter is added to the reload check interval. This is jitter from the
perspective of multiple CoreDNS instances; each instance still checks on a
regular interval, but all of these instances will have their reloads spread
out across the jitter duration. This isn't strictly necessary given that the
reloads are graceful, and can be disabled by setting the jitter to `0s`.

Jitter is re-calculated whenever the Corefile is reloaded.

## Syntax

~~~ txt
reload [INTERVAL] [JITTER]
~~~

* The plugin will check for changes every **INTERVAL**, subject to +/- the **JITTER** duration
* **INTERVAL** and **JITTER** are Golang (durations)[https://golang.org/pkg/time/#ParseDuration]
* Default **INTERVAL** is 30s, default **JITTER** is 15s
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
