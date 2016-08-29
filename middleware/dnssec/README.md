# dnssec

`dnssec` enables on-the-fly DNSSEC signing of served data.

## Syntax

~~~
dnssec [zones...]
~~~

* `zones` zones that should be signed. If empty, the zones from the configuration block
    are used.

If keys are not specified (see below), a key is generated and used for all signing operations. The
DNSSEC signing will treat this key a CSK (common signing key), forgoing the ZSK/KSK split. All
signing operations are done online. Authenticated denial of existence is implemented with NSEC black
lies. Using ECDSA as an algorithm is preferred as this leads to smaller signatures (compared to
RSA). NSEC3 is *not* supported.

A signing key can be specified by using the `key` directive.

NOTE: Key generation has not been implemented yet.

TODO(miek): think about key rollovers, and how to do them automatically.


~~~
dnssec [zones... ] {
    key file [key...]
}
~~~

* `key file` indicates that key file(s) should be read from disk. When multiple keys are specified, RRsets
  will be signed with all keys. Generating a key can be done with `dnssec-keygen`: `dnssec-keygen -a
  ECDSAP256SHA256 <zonename>`. A key created for zone *A* can be safely used for zone *B*.

## Examples
