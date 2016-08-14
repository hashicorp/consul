# dnssec

`dnssec` enables on-the-fly DNSSEC signing of served data.

## Syntax

~~~
dnssec [zones...]
~~~

* `zones` zones that should be signed. If empty the zones from the configuration block
    are used.

If keys are not specified (see below) a key is generated and used for all signing operations. The
DNSSEC signing will treat this key a CSK (common signing key) forgoing the ZSK/KSK split. All
signing operations are done online. Authenticated denial of existence is implemented with NSEC black
lies. Using ECDSA as an algorithm is preferred as this leads to smaller signatures (compared to
RSA).

A signing key can be specified by using the `key` directive.

WARNING: when a key is generated there is currently no way to extract any key material from CoreDNS,
this key only lives in memory. See issue <https://github.com/miekg/coredns/issues/211>.

TODO(miek): think about key rollovers.


~~~
dnssec [zones... ] {
    key file [key...]
}
~~~

* `key file` indicates key file(s) should be read from disk. When multiple keys are specified, RRset
  will be signed with all keys. Generating a key can be done with `dnssec-keygen`: `dnssec-keygen -a
  ECDSAP256SHA256 <zonename>`. A key created for zone *A* can be safely used for zone *B*.

## Examples
