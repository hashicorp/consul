# loop

## Name

*loop* - detect simple forwarding loops and halt the server.

## Description

The *loop* plugin will send a random probe query to ourselves and will then keep track of how many times
we see it. If we see it more than twice, we assume CoreDNS has seen a forwarding loop and we halt the process.

The plugin will try to send the query for up to 30 seconds. This is done to give CoreDNS enough time
to start up. Once a query has been successfully sent, *loop* disables itself to prevent a query of
death.

The query sent is `<random number>.<random number>.zone` with type set to HINFO.

## Syntax

~~~ txt
loop
~~~

## Examples

Start a server on the default port and load the *loop* and *forward* plugins. The *forward* plugin
forwards to it self.

~~~ txt
. {
    loop
    forward . 127.0.0.1
}
~~~

After CoreDNS has started it stops the process while logging:

~~~ txt
plugin/loop: Loop (127.0.0.1:55953 -> :1053) detected for zone ".", see https://coredns.io/plugins/loop#troubleshooting. Query: "HINFO 4547991504243258144.3688648895315093531."
~~~

## Limitations

This plugin only attempts to find simple static forwarding loops at start up time. To detect a loop,
the following must be true:

*  the loop must be present at start up time.

*  the loop must occur for the `HINFO` query type.

## Troubleshooting

When CoreDNS logs contain the message `Loop ... detected ...`, this means that the `loop` detection
plugin has detected an infinite forwarding loop in one of the upstream DNS servers. This is a fatal
error because operating with an infinite loop will consume memory and CPU until eventual out of
memory death by the host.

A forwarding loop is usually caused by:

* Most commonly, CoreDNS forwarding requests directly to itself. e.g. via a loopback address such as `127.0.0.1`, `::1` or `127.0.0.53`
* Less commonly, CoreDNS forwarding to an upstream server that in turn, forwards requests back to CoreDNS.

To troubleshoot this problem, look in your Corefile for any `proxy` or `forward` to the zone
in which the loop was detected.  Make sure that they are not forwarding to a local address or
to another DNS server that is forwarding requests back to CoreDNS. If `proxy` or `forward` are
 using a file (e.g. `/etc/resolv.conf`), make sure that file does not contain local addresses.

### Troubleshooting Loops In Kubernetes Clusters

When a CoreDNS Pod deployed in Kubernetes detects a loop, the CoreDNS Pod will start to "CrashLoopBackOff".
This is because Kubernetes will try to restart the Pod every time CoreDNS detects the loop and exits.

A common cause of forwarding loops in Kubernetes clusters is an interaction with a local DNS cache
on the host node (e.g. `systemd-resolved`).  For example, in certain configurations `systemd-resolved` will
put the loopback address `127.0.0.53` as a nameserver into `/etc/resolv.conf`. Kubernetes (via `kubelet`) by default
will pass this `/etc/resolv.conf` file to all Pods using the `default` dnsPolicy rendering them
unable to make DNS lookups (this includes CoreDNS Pods). CoreDNS uses this `/etc/resolv.conf`
as a list of upstreams to proxy/forward requests to.  Since it contains a loopback address, CoreDNS ends up forwarding
requests to itself.

There are many ways to work around this issue, some are listed here:

* Add the following to `kubelet`: `--resolv-conf <path-to-your-real-resolv-conf-file>`.  Your "real"
  `resolv.conf` is the one that contains the actual IPs of your upstream servers, and no local/loopback address.
  This flag tells `kubelet` to pass an alternate `resolv.conf` to Pods. For systems using `systemd-resolved`,
`/run/systemd/resolve/resolv.conf` is typically the location of the "real" `resolv.conf`,
although this can be different depending on your distribution.
* Disable the local DNS cache on host nodes, and restore `/etc/resolv.conf` to the original.
* A quick and dirty fix is to edit your Corefile, replacing `proxy . /etc/resolv.conf` with
the ip address of your upstream DNS, for example `proxy . 8.8.8.8`.  But this only fixes the issue for CoreDNS,
kubelet will continue to forward the invalid `resolv.conf` to all `default` dnsPolicy Pods, leaving them unable to resolve DNS.
