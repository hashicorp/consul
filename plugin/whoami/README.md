# whoami

*whoami* returns your resolver's local IP address, port and transport. Your IP address is returned
 in the additional section as either an A or AAAA record.

When CoreDNS can not find a Corefile to load, this is the default plugin it loads.

The reply always has an empty answer section. The port and transport are included in the additional
section as a SRV record, transport can be "tcp" or "udp".

~~~ txt
._<transport>.qname. 0 IN SRV 0 0 <port> .
~~~

If CoreDNS can't find a Corefile on startup this is the *default* plugin that gets loaded. As
such it can be used to check that CoreDNS is responding to queries. Other than that this plugin
is of limited use in production.

The *whoami* plugin will respond to every A or AAAA query, regardless of the query name.

## Syntax

~~~ txt
whoami
~~~

## Examples

Start a server on the default port and load the *whoami* plugin.

~~~ corefile
. {
    whoami
}
~~~

When queried for "example.org A", CoreDNS will respond with:

~~~ txt
;; QUESTION SECTION:
;example.org.   IN       A

;; ADDITIONAL SECTION:
example.org.            0       IN      A       10.240.0.1
_udp.example.org.       0       IN      SRV     0 0 40212
~~~
