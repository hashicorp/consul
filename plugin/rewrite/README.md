# rewrite

## Name

*rewrite* - performs internal message rewriting.

## Description

Rewrites are invisible to the client. There are simple rewrites (fast) and complex rewrites
(slower), but they're powerful enough to accommodate most dynamic back-end applications.

## Syntax

~~~
rewrite [continue|stop] FIELD FROM TO
~~~

* **FIELD** is (`type`, `class`, `name`, ...)
* **FROM** is the exact name of type to match
* **TO** is the destination name or type to rewrite to

When the FIELD is `type` and FROM is (`A`, `MX`, etc.), the type of the message will be rewritten;
e.g., to rewrite ANY queries to HINFO, use `rewrite type ANY HINFO`.

When the FIELD is `class` and FROM is (`IN`, `CH`, or `HS`) the class of the message will be
rewritten; e.g., to rewrite CH queries to IN use `rewrite class CH IN`.

When the FIELD is `name` the query name in the message is rewritten; this
needs to be a full match of the name, e.g., `rewrite name miek.nl example.org`.

When the FIELD is `edns0` an EDNS0 option can be appended to the request as described below.

If you specify multiple rules and an incoming query matches on multiple rules, the rewrite
will behave as following
* `continue` will continue apply the next rule in the rule list.
* `stop` will consider the current rule is the last rule and will not continue.  Default behaviour
for not specifying this rule processing mode is `stop`

### Name Field Rewrites

The `rewrite` plugin offers the ability to match on the name in the question section of
a DNS request. The match could be exact, substring, or based on a prefix, suffix, or regular
expression.

The syntax for the name re-writing is as follows:

```
rewrite [continue|stop] name [exact|prefix|suffix|substring|regex] STRING STRING
```

The match type, i.e. `exact`, `substring`, etc., triggers re-write:

* **exact** (default): on exact match of the name in the question section of a request
* **substring**: on a partial match of the name in the question section of a request
* **prefix**: when the name begins with the matching string
* **suffix**: when the name ends with the matching string
* **regex**: when the name in the question section of a request matches a regular expression

If the match type is omitted, the `exact` match type is being assumed.

The following instruction allows re-writing the name in the query that
contains `service.us-west-1.example.org` substring.

```
rewrite name substring service.us-west-1.example.org service.us-west-1.consul
```

Thus:

* Incoming Request Name: `ftp.service.us-west-1.example.org`
* Re-written Request Name: `ftp.service.us-west-1.consul`

The following instruction uses regular expressions. The name in a request
matching `(.*)-(us-west-1)\.example\.org` regular expression is being replaces with
`{1}.service.{2}.consul`, where `{1}` and `{2}` are regular expression match groups.

```
rewrite name regex (.*)-(us-west-1)\.example\.org {1}.service.{2}.consul
```

Thus:

* Incoming Request Name: `ftp-us-west-1.example.org`
* Re-written Request Name: `ftp.service.us-west-1.consul`

### Response Rewrites

When re-writing incoming DNS requests' names, CoreDNS re-writes the `QUESTION SECTION`
section of the requests. It may be necessary to re-write the `ANSWER SECTION` of the
requests, because some DNS resolvers would treat the mismatch between `QUESTION SECTION`
and `ANSWER SECTION` as a man-in-the-middle attack (MITM).

For example, a user tries to resolve `ftp-us-west-1.coredns.rocks`. The
CoreDNS configuration file has the following rule:

```
rewrite name regex (.*)-(us-west-1)\.coredns\.rocks {1}.service.{2}.consul
```

CoreDNS instance re-wrote the request to `ftp-us-west-1.coredns.rocks` with
`ftp.service.us-west-1.consul` and ultimately resolved it to 3 records.
The resolved records, see `ANSWER SECTION`, were not from `coredns.rocks`, but
rather from `service.us-west-1.consul`.


```
$ dig @10.1.1.1 ftp-us-west-1.coredns.rocks

; <<>> DiG 9.8.3-P1 <<>> @10.1.1.1 ftp-us-west-1.coredns.rocks
; (1 server found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 8619
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 3, AUTHORITY: 0, ADDITIONAL: 0

;; QUESTION SECTION:
;ftp-us-west-1.coredns.rocks. IN A

;; ANSWER SECTION:
ftp.service.us-west-1.consul. 0    IN A    10.10.10.10
ftp.service.us-west-1.consul. 0    IN A    10.20.20.20
ftp.service.us-west-1.consul. 0    IN A    10.30.30.30
```

The above is the mismatch.

The following configuration snippet allows for the re-writing of the
`ANSWER SECTION`, provided that the `QUESTION SECTION` was re-written:

```
    rewrite stop {
        name regex (.*)-(us-west-1)\.coredns\.rocks {1}.service.{2}.consul
        answer name (.*)\.service\.(us-west-1)\.consul {1}-{2}.coredns.rocks
    }
```

Now, the `ANSWER SECTION` matches the `QUESTION SECTION`:

```
$ dig @10.1.1.1 ftp-us-west-1.coredns.rocks

; <<>> DiG 9.8.3-P1 <<>> @10.1.1.1 ftp-us-west-1.coredns.rocks
; (1 server found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 8619
;; flags: qr aa rd ra; QUERY: 1, ANSWER: 3, AUTHORITY: 0, ADDITIONAL: 0

;; QUESTION SECTION:
;ftp-us-west-1.coredns.rocks. IN A

;; ANSWER SECTION:
ftp-us-west-1.coredns.rocks. 0    IN A    10.10.10.10
ftp-us-west-1.coredns.rocks. 0    IN A    10.20.20.20
ftp-us-west-1.coredns.rocks. 0    IN A    10.30.30.30
```

The syntax for the response of DNS request and response is as follows:

```
rewrite [continue|stop] {
    name regex STRING STRING
    answer name STRING STRING
}
```

## EDNS0 Options

Using FIELD edns0, you can set, append, or replace specific EDNS0 options on the request.

* `replace` will modify any matching (what that means may vary based on EDNS0 type) option with the specified option
* `append` will add the option regardless of what options already exist
* `set` will modify a matching option or add one if none is found

Currently supported are `EDNS0_LOCAL`, `EDNS0_NSID` and `EDNS0_SUBNET`.

### EDNS0_LOCAL

This has two fields, code and data. A match is defined as having the same code. Data may be a string or a variable.

* A string data can be treated as hex if it starts with `0x`. Example:

~~~ corefile
. {
    rewrite edns0 local set 0xffee 0x61626364
    whoami
}
~~~

rewrites the first local option with code 0xffee, setting the data to "abcd". Equivalent:

~~~ corefile
. {
    rewrite edns0 local set 0xffee abcd
}
~~~

* A variable data is specified with a pair of curly brackets `{}`. Following are the supported variables:
  {qname}, {qtype}, {client_ip}, {client_port}, {protocol}, {server_ip}, {server_port}.

Example:

~~~
rewrite edns0 local set 0xffee {client_ip}
~~~

### EDNS0_NSID

This has no fields; it will add an NSID option with an empty string for the NSID. If the option already exists
and the action is `replace` or `set`, then the NSID in the option will be set to the empty string.

### EDNS0_SUBNET

This has two fields,  IPv4 bitmask length and IPv6 bitmask length. The bitmask
length is used to extract the client subnet from the source IP address in the query.

Example:

~~~
rewrite edns0 subnet set 24 56
~~~

* If the query has source IP as IPv4, the first 24 bits in the IP will be the network subnet.
* If the query has source IP as IPv6, the first 56 bits in the IP will be the network subnet.
