# template

## Name

*template* - allows for dynamic responses based on the incoming query.

## Description

The *template* plugin allows you to dynamically respond to queries by just writing a (Go) template.

## Syntax

~~~
template CLASS TYPE [ZONE...] {
    [match REGEX...]
    [answer RR]
    [additional RR]
    [authority RR]
    [...]
    [rcode CODE]
    [fallthrough [ZONE...]]
}
~~~

* **CLASS** the query class (usually IN or ANY).
* **TYPE** the query type (A, PTR, ... can be ANY to match all types).
* **ZONE** the zone scope(s) for this template. Defaults to the server zones.
* **REGEX** [Go regexp](https://golang.org/pkg/regexp/) that are matched against the incoming question name. Specifying no regex matches everything (default: `.*`). First matching regex wins.
* `answer|additional|authority` **RR** A [RFC 1035](https://tools.ietf.org/html/rfc1035#section-5) style resource record fragment
  built by a [Go template](https://golang.org/pkg/text/template/) that contains the reply.
* `rcode` **CODE** A response code (`NXDOMAIN, SERVFAIL, ...`). The default is `SUCCESS`.
* `fallthrough` Continue with the next plugin if the zone matched but no regex matched.
  If specific zones are listed (for example `in-addr.arpa` and `ip6.arpa`), then only queries for
  those zones will be subject to fallthrough.

At least one `answer` or `rcode` directive is needed (e.g. `rcode NXDOMAIN`).

[Also see](#also-see) contains an additional reading list.

## Templates

Each resource record is a full-featured [Go template](https://golang.org/pkg/text/template/) with the following predefined data

* `.Zone` the matched zone string (e.g. `example.`).
* `.Name` the query name, as a string (lowercased).
* `.Class` the query class (usually `IN`).
* `.Type` the RR type requested (e.g. `PTR`).
* `.Match` an array of all matches. `index .Match 0` refers to the whole match.
* `.Group` a map of the named capture groups.
* `.Message` the complete incoming DNS message.
* `.Question` the matched question section.

The output of the template must be a [RFC 1035](https://tools.ietf.org/html/rfc1035) style resource record (commonly referred to as a "zone file").

**WARNING** there is a syntactical problem with Go templates and CoreDNS config files. Expressions
 like `{{$var}}` will be interpreted as a reference to an environment variable by CoreDNS (and
 Caddy) while `{{ $var }}` will work. See [Bugs](#bugs) and corefile(5).

## Metrics

If monitoring is enabled (via the *prometheus* directive) then the following metrics are exported:

* `coredns_template_matches_total{regex}` the total number of matched requests by regex.
* `coredns_template_template_failures_total{regex,section,template}` the number of times the Go templating failed. Regex, section and template label values can be used to map the error back to the config file.
* `coredns_template_rr_failures_total{regex,section,template}` the number of times the templated resource record was invalid and could not be parsed. Regex, section and template label values can be used to map the error back to the config file.

Both failure cases indicate a problem with the template configuration.

## Examples

### Resolve everything to NXDOMAIN

The most simplistic template is

~~~ corefile
. {
    template ANY ANY {
      rcode NXDOMAIN
    }
}
~~~

1. This template uses the default zone (`.` or all queries)
2. All queries will be answered (no `fallthrough`)
3. The answer is always NXDOMAIN

### Resolve .invalid as NXDOMAIN

The `.invalid` domain is a reserved TLD (see [RFC-2606 Reserved Top Level DNS Names](https://tools.ietf.org/html/rfc2606#section-2)) to indicate invalid domains.

~~~ corefile
. {
    proxy . 8.8.8.8

    template ANY ANY invalid {
      rcode NXDOMAIN
      authority "invalid. 60 {{ .Class }} SOA ns.invalid. hostmaster.invalid. (1 60 60 60 60)"
    }
}
~~~

1. A query to .invalid will result in NXDOMAIN (rcode)
2. A dummy SOA record is sent to hand out a TTL of 60s for caching purposes
3. Querying `.invalid` in the `CH` class will also cause a NXDOMAIN/SOA response
4. The default regex is `.*`

### Block invalid search domain completions

Imagine you run `example.com` with a datacenter `dc1.example.com`. The datacenter domain
is part of the DNS search domain.
However `something.example.com.dc1.example.com` would indicate a fully qualified
domain name (`something.example.com`) that inadvertently has the default domain or search
path (`dc1.example.com`) added.

~~~ corefile
. {
    proxy . 8.8.8.8

    template IN ANY example.com.dc1.example.com {
      rcode NXDOMAIN
      authority "{{ .Zone }} 60 IN SOA ns.example.com hostmaster.example.com (1 60 60 60 60)"
    }
}
~~~

A more verbose regex based equivalent would be

~~~ corefile
. {
    proxy . 8.8.8.8

    template IN ANY example.com {
      match "example\.com\.(dc1\.example\.com\.)$"
      rcode NXDOMAIN
      authority "{{ index .Match 1 }} 60 IN SOA ns.{{ index .Match 1 }} hostmaster.{{ index .Match 1 }} (1 60 60 60 60)"
      fallthrough
    }
}
~~~

The regex-based version can do more complex matching/templating while zone-based templating is easier to read and use.

### Resolve A/PTR for .example

~~~ corefile
. {
    proxy . 8.8.8.8

    # ip-a-b-c-d.example.com A a.b.c.d

    template IN A example {
      match (^|[.])ip-10-(?P<b>[0-9]*)-(?P<c>[0-9]*)-(?P<d>[0-9]*)[.]example[.]$
      answer "{{ .Name }} 60 IN A 10.{{ .Group.b }}.{{ .Group.c }}.{{ .Group.d }}"
      fallthrough
    }

    # d.c.b.a.in-addr.arpa PTR ip-a-b-c-d.example

    template IN PTR 10.in-addr.arpa. {
      match ^(?P<d>[0-9]*)[.](?P<c>[0-9]*)[.](?P<b>[0-9]*)[.]10[.]in-addr[.]arpa[.]$
      answer "{{ .Name }} 60 IN PTR ip-10-{{ .Group.b }}-{{ .Group.c }}-{{ .Group.d }}.example.com."
    }
}
~~~

An IPv4 address consists of 4 bytes, `a.b.c.d`. Named groups make it less error-prone to reverse the
IP address in the PTR case. Try to use named groups to explain what your regex and template are doing.

Note that the A record is actually a wildcard: any subdomain of the IP address will resolve to the IP address.

Having templates to map certain PTR/A pairs is a common pattern.

Fallthrough is needed for mixed domains where only some responses are templated.

### Resolve multiple ip patterns

~~~ corefile
. {
    proxy . 8.8.8.8

    template IN A example {
      match "^ip-(?P<a>10)-(?P<b>[0-9]*)-(?P<c>[0-9]*)-(?P<d>[0-9]*)[.]dc[.]example[.]$"
      match "^(?P<a>[0-9]*)[.](?P<b>[0-9]*)[.](?P<c>[0-9]*)[.](?P<d>[0-9]*)[.]ext[.]example[.]$"
      answer "{{ .Name }} 60 IN A {{ .Group.a}}.{{ .Group.b }}.{{ .Group.c }}.{{ .Group.d }}"
      fallthrough
    }
}
~~~

Named capture groups can be used to template one response for multiple patterns.

### Resolve A and MX records for IP templates in .example

~~~ corefile
. {
    proxy . 8.8.8.8

    template IN A example {
      match ^ip-10-(?P<b>[0-9]*)-(?P<c>[0-9]*)-(?P<d>[0-9]*)[.]example[.]$
      answer "{{ .Name }} 60 IN A 10.{{ .Group.b }}.{{ .Group.c }}.{{ .Group.d }}"
      fallthrough
    }
    template IN MX example {
      match ^ip-10-(?P<b>[0-9]*)-(?P<c>[0-9]*)-(?P<d>[0-9]*)[.]example[.]$
      answer "{{ .Name }} 60 IN MX 10 {{ .Name }}"
      additional "{{ .Name }} 60 IN A 10.{{ .Group.b }}.{{ .Group.c }}.{{ .Group.d }}"
      fallthrough
    }
}
~~~

### Adding authoritative nameservers to the response

~~~ corefile
. {
    proxy . 8.8.8.8

    template IN A example {
      match ^ip-10-(?P<b>[0-9]*)-(?P<c>[0-9]*)-(?P<d>[0-9]*)[.]example[.]$
      answer "{{ .Name }} 60 IN A 10.{{ .Group.b }}.{{ .Group.c }}.{{ .Group.d }}"
      authority  "example. 60 IN NS ns0.example."
      authority  "example. 60 IN NS ns1.example."
      additional "ns0.example. 60 IN A 203.0.113.8"
      additional "ns1.example. 60 IN A 198.51.100.8"
      fallthrough
    }
    template IN MX example {
      match ^ip-10-(?P<b>[0-9]*)-(?P<c>[0-9]*)-(?P<d>[0-9]*)[.]example[.]$
      answer "{{ .Name }} 60 IN MX 10 {{ .Name }}"
      additional "{{ .Name }} 60 IN A 10.{{ .Group.b }}.{{ .Group.c }}.{{ .Group.d }}"
      authority  "example. 60 IN NS ns0.example."
      authority  "example. 60 IN NS ns1.example."
      additional "ns0.example. 60 IN A 203.0.113.8"
      additional "ns1.example. 60 IN A 198.51.100.8"
      fallthrough
    }
}
~~~

## Also see

* [Go regexp](https://golang.org/pkg/regexp/) for details about the regex implementation
* [RE2 syntax reference](https://github.com/google/re2/wiki/Syntax) for details about the regex syntax
* [RFC-1034](https://tools.ietf.org/html/rfc1034#section-3.6.1) and [RFC 1035](https://tools.ietf.org/html/rfc1035#section-5) for the resource record format
* [Go template](https://golang.org/pkg/text/template/) for the template language reference

## Bugs

CoreDNS supports [caddyfile environment variables](https://caddyserver.com/docs/caddyfile#env)
with notion of `{$ENV_VAR}`. This parser feature will break [Go template variables](https://golang.org/pkg/text/template/#hdr-Variables) notations like`{{$variable}}`.
The equivalent notation `{{ $variable }}` will work.
Try to avoid Go template variables in the context of this plugin.
