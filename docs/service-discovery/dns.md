# DNS Interface

The DNS interface allows users to find the IP address and port of services, using DNS
queries. The DNS interface is in many ways similar to an HTTP API. The major difference is
in the DNS protocol.

There are lots of guides to DNS, the following list is a short reference that should help you
understand the parts that are relevant to the DNS interface in Consul. Full details about
the DNS protocol can be found in the RFCs: [RFC 1035], [RFC 6891], [RFC 2782], and others.

[RFC 1035]: https://tools.ietf.org/html/rfc1035
[RFC 6891]: https://tools.ietf.org/html/rfc6891
[RFC 2782]: https://tools.ietf.org/html/rfc2782

* [wikipedia: DNS message format](https://en.wikipedia.org/wiki/Domain_Name_System#DNS_message_format)
  is a quick introduction to the format used for queries and replies
* [RFC 1035 Section 4.1.1](https://datatracker.ietf.org/doc/html/rfc1035#section-4.1.1)
  is a good reference for when to use specific response codes and what the different header
  bits refer to. 


## DNS Server

The DNS interface is implemented as a DNS server using [miekg/dns] and the handlers for
requests are in `agent/dns.go`.

[miekg/dns]: https://github.com/miekg/dns

## DNS Queries and Records

The DNS interface handles queries where OPCODE=0 (standard query). The following table describe the current
DNS behaviour for different record types and domain names. The Domain names (along the
top) are always in the form `<prefix>.<name in table>.<domain>`, where `domain` is
generally documented as `consul` but can be set using the `domain` or `alt_domain` config
fields. The `prefix` is an identifier (service name, node name, prepared query name, etc).

| Type    | service                 | connect                 | ingress                 | node                   | query           | addr                         |
|---------|-------------------------|-------------------------|-------------------------|------------------------|-----------------|------------------------------|
| SOA     | Supported               | Supported               | Supported               | Supported              | Supported       | Supported                    |
| NS      | Supported               | Supported               | Supported               | Supported              | Supported       | Supported                    |
| AXFR    | Not Implemented         | Not Implemented         | Not Implemented         | Not Implemented        | Not Implemented | Not Implemented              |
| A/AAAA  | Supported               | Supported               | Supported               | Supported              |                 | Supported                    |
| ANY     | Supported (return A)    | Supported (return A)    | Supported (return A)    | Supported              |                 | Supported (return A)         |
| CNAME   | Supported (node cname)  | Supported (node cname)  | Supported (node cname)  | Supported (node cname) |                 | return empty with A as extra |
| OPT     | Supported (node OPT)    | Supported (node OPT)    | Supported (node OPT)    | Supported (node OPT)   |                 | return empty with A as extra |
| PTR     | Supported (node PTR)    | Supported (node PTR)    | Supported (node PTR)    | Supported (node PTR)   |                 | return empty with A as extra |
| SRV     | Supported (service SRV) | Supported (service SRV) | Supported (service SRV) | No error but empty     |                 | return empty with A as extra |
| TXT     | Answer A record (????)  | Answer A record (????)  | Answer A record (????)  | Supported              |                 | return empty with A as extra |

