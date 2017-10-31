# dnssec

*dnssec* enables on-the-fly DNSSEC signing of served data.

## Syntax

~~~
dnssec [ZONES... ] {
    key file KEY...
    cache_capacity CAPACITY
}
~~~

The specified key is used for all signing operations. The DNSSEC signing will treat this key a
CSK (common signing key), forgoing the ZSK/KSK split. All signing operations are done online.
Authenticated denial of existence is implemented with NSEC black lies. Using ECDSA as an algorithm
is preferred as this leads to smaller signatures (compared to RSA). NSEC3 is *not* supported.

If multiple *dnssec* plugins are specified in the same zone, the last one specified will be
used (See [bugs](#bugs)).

* **ZONES** zones that should be signed. If empty, the zones from the configuration block
    are used.

* `key file` indicates that **KEY** file(s) should be read from disk. When multiple keys are specified, RRsets
  will be signed with all keys. Generating a key can be done with `dnssec-keygen`: `dnssec-keygen -a
  ECDSAP256SHA256 <zonename>`. A key created for zone *A* can be safely used for zone *B*. The name of the
  key file can be specified as one of the following formats

    * basename of the generated key `Kexample.org+013+45330`
    * generated public key `Kexample.org+013+45330.key`
    * generated private key `Kexample.org+013+45330.private`

* `cache_capacity` indicates the capacity of the cache. The dnssec plugin uses a cache to store
  RRSIGs. The default for **CAPACITY** is 10000.

## Metrics

If monitoring is enabled (via the *prometheus* directive) then the following metrics are exported:

* `coredns_dnssec_cache_size{type}` - total elements in the cache, type is "signature".
* `coredns_dnssec_cache_capacity{type}` - total capacity of the cache, type is "signature".
* `coredns_dnssec_cache_hits_total{}` - Counter of cache hits.
* `coredns_dnssec_cache_misses_total{}` - Counter of cache misses.

## Examples

Sign responses for `example.org` with the key "Kexample.org.+013+45330.key".

~~~ corefile
example.org {
    dnssec {
        key file Kexample.org.+013+45330
    }
    whoami
}
~~~

Sign responses for a kubernetes zone with the key "Kcluster.local+013+45129.key".

~~~
cluster.local {
    kubernetes
    dnssec {
      key file Kcluster.local+013+45129
    }
}
~~~

## Bugs

Multiple *dnssec* plugins inside one server stanza will silently overwrite earlier ones, here
`example.local` will overwrite the one for `cluster.org`.

~~~
. {
    kubernetes cluster.local
    dnssec cluster.local {
      key file Kcluster.local+013+45129
    }
    dnssec example.org {
      key file Kexample.org.+013+45330
    }
}
~~~
