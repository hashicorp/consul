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

The following table describe the current DNS behaviour depending on the dns query kind (node, service...), and the query type (A/AAAA, SRV...)

|                | service                 | connect (enterprise)    | ingress (enterprise)    | node                   | query           | addr                         |
|----------------|-------------------------|-------------------------|-------------------------|------------------------|-----------------|------------------------------|
| TypeSOA        | Supported               | Supported               | Supported               | Supported              | Supported       | Supported                    |
| TypeNS         | Supported               | Supported               | Supported               | Supported              | Supported       | Supported                    |
| TypeAXFR       | Not Implemented         | Not Implemented         | Not Implemented         | Not Implemented        | Not Implemented | Not Implemented              |
| TypeA/TypeAAAA | Supported               | Supported               | Supported               | Supported              |                 | Supported                    |
| TypeANY        | Supported (return A)    | Supported (return A)    | Supported (return A)    | Supported              |                 | Supported (return A)         |
| TypeCNAME      | Supported (node cname)  | Supported (node cname)  | Supported (node cname)  | Supported (node cname) |                 | return empty with A as extra |
| TypeOPT        | Supported (node OPT)    | Supported (node OPT)    | Supported (node OPT)    | Supported (node OPT)   |                 | return empty with A as extra |
| TypePTR        | Supported (node PTR)    | Supported (node PTR)    | Supported (node PTR)    | Supported (node PTR)   |                 | return empty with A as extra |
| TypeSRV        | Supported (service SRV) | Supported (service SRV) | Supported (service SRV) | No error but empty     |                 | return empty with A as extra |
| TypeTXT        | Answer A record (????)  | Answer A record (????)  | Answer A record (????)  | Supported              |                 | return empty with A as extra |

[miekg/dns]: https://github.com/miekg/dns
