# nsid

## Name

*nsid* - adds an identifier of this server to each reply.

## Description

This plugin implements RFC 5001 and adds an EDNS0 OPT resource record to replies that uniquely
identify the server. This is useful in anycast setups to see which server was responsible for
generating the reply and for debugging.

## Syntax

~~~ txt
nsid [DATA]
~~~

**DATA** is the string to use in the nsid record.

If **DATA** is not given, the host's name is used.

## Examples

Enable nsid:

~~~ corefile
. {
    whoami
    nsid Use The Force
}
~~~

And now a client with NSID support will see an OPT record with the NSID option:

~~~ sh
% dig +nsid @localhost a whoami.example.org

;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 46880
;; flags: qr aa rd; QUERY: 1, ANSWER: 0, AUTHORITY: 0, ADDITIONAL: 3

....

; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 4096
; NSID: 55 73 65 20 54 68 65 20 46 6f 72 63 65 ("Use The Force")
;; QUESTION SECTION:
;whoami.example.org.		IN	A
~~~
