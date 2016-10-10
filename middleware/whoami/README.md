# whoami

*whoami* returns your local IP address, port and transport used. Your local IP address is returned in
the additional section as either an A or AAAA record.

The port and transport are included in the additional section as a SRV record, transport can be
"tcp" or "udp".

~~~ txt
._<transport>.qname. 0 IN SRV 0 0 <port> .
~~~

The *whoami* middleware will respond to every A or AAAA query, regardless of the query name.

## Syntax

~~~ txt
whoami
~~~

## Examples

~~~ txt
.:53 {
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
