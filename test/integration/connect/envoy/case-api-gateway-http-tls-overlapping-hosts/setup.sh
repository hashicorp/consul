#!/bin/bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0


set -euo pipefail

upsert_config_entry primary '
kind = "api-gateway"
name = "api-gateway"
listeners = [
  {
    name = "listener-one"
    port = 9999
    protocol = "http"
    tls = {
      certificates = [
        {
          kind = "inline-certificate"
          name = "host-consul-example"
        }
      ]
    }
  },
  {
    name = "listener-two"
    port = 9998
    protocol = "http"
    tls = {
      certificates = [
        {
          kind = "inline-certificate"
          name = "host-consul-example"
        },
        {
          kind = "inline-certificate"
          name = "also-host-consul-example"
        },
        { 
          kind = "inline-certificate"
          name = "other-consul-example"
        }
      ]
    }
  }
]
'

upsert_config_entry primary '
kind = "inline-certificate"
name = "host-consul-example"
private_key = <<EOF
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0wzZeonUklhOvJ0AxcdDdCTiMwR9tsm/6IGcw9Jm50xVY+qg
5GFg1RWrQaODq7Gjqd/JDUAwtTBnQMs1yt6nbsHe2QhbD4XeqtZ+6fTv1ZpG3k8F
eB/M01xFqovczRV/ie77wd4vqoPD+AcfD8NDAFJt3htwUgGIqkQHP329Sh3TtLga
9ZMCs1MoTT+POYGUPL8bwt9R6ClNrucbH4Bs6OnX2ZFbKF75O9OHKNxWTmpDSodv
OFbFyKps3BfnPuF0Z6mj5M5yZeCjmtfS25PrsM3pMBGK5YHb0MlFfZIrIGboMbrz
9F/BMQJ64pMe43KwqHvTnbKWhp6PzLhEkPGLnwIDAQABAoIBADBEJAiONPszDu67
yU1yAM8zEDgysr127liyK7PtDnOfVXgAVMNmMcsJpZzhVF+TxKY487YAFCOb6kE7
OBYpTYla9SgVbR3js8TGQUgoKCFlowd8cvfB7gn4dEZIrjqIzB4zdYgk1Cne8JZs
qoHkWhJcx5ugEtPuXd7yp+WxT/T+6uOro06scp67NhP5t9yoAGFv5Vdb577RuzRo
Wkd9higQ9A20+GtjCY0EYxdgRviWvW7mM5/F+Lzcaui86ME+ga754gX8zgW3+NJ5
LMsz5OLSnh291Uyjmr77HWBv/xvpq01Fls0LyJcgxFVZuJs5GQz+l3otSqv4FTP6
Ua9w/YECgYEA8To3dgUK1QhzX5rwhWtlst3pItGTvmEdNzXmjgSylu7uKM13i+xg
llhp2uXrOEtuL+xtBZdeFNaijusbyqjg0xj6e4o31c19okuuDkJD5/sfQq22bvrn
gVJMGuESprIiPePrEyrXCHOdxH6eDgR2dIzAeO5vz0nnKGFAWrJJbvECgYEA3/mJ
eacXOJznw4Sa8jGWS2FtZLKxDHph7uDKMJmuG0ukb3aHJ9dMHrPleCLo8mhpoObA
hueoIbIP7swGrQx79+nZbnQpF6rMp6FAU5bF3gSrj1eWbaeh8pn9mrv4hal9USmn
orTbXMxDp3XSh7voR8Fqy5tMQqwZ+Lz74ccbw48CgYEA5cEhGdNrocPOv3x/IVRN
JLOfXX5nTaiJfxBja1imEIO5ajtoZWjaBdhn2gmqo4+UfyicHfsxrH9RjPX5HmkC
2Yys5gWbcJOr2Wxjd0k+DDFucL+rRsDKxq1vtxov/X0kh/YQ68ydynr0BTbjq04s
1I1KtOPEspYdCKS3+qpcrsECgYBtvYeVesBO9do9G0kMKC26y4bdEwzaz1ASykNn
IrWDHEH6dznr1HqwhHaHsZsvwucWdlmZAAKKWAOkfoU63uYS55qomvPTa9WQwNqS
2koi6Wjh+Al1uvAHvVncKgOwAgar8Nv5ReJBirgPYhSAexpppiRclL/93vNuw7Iq
wvMgkwKBgQC5wnb6SUUrzzKKSRgyusHM/XrjiKgVKq7lvFE9/iJkcw+BEXpjjbEe
RyD0a7PRtCfR39SMVrZp4KXVNNK5ln0WhuLvraMDwOpH9JDWHQiAhuJ3ooSwBylK
+QCLjyOtWAGZAIBRJyb1txfTXZ++dldkOjBi3bmEiadOa48ksvDsNQ==
-----END RSA PRIVATE KEY-----
EOF

certificate = <<EOF
-----BEGIN CERTIFICATE-----
MIIDQjCCAioCCQC6cMRYsE+ahDANBgkqhkiG9w0BAQsFADBjMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCQ0ExCzAJBgNVBAcMAkxBMQ0wCwYDVQQKDARUZXN0MQ0wCwYD
VQQLDARTdHViMRwwGgYDVQQDDBNob3N0LmNvbnN1bC5leGFtcGxlMB4XDTIzMDIx
NzAyMTA1MloXDTI4MDIxNjAyMTA1MlowYzELMAkGA1UEBhMCVVMxCzAJBgNVBAgM
AkNBMQswCQYDVQQHDAJMQTENMAsGA1UECgwEVGVzdDENMAsGA1UECwwEU3R1YjEc
MBoGA1UEAwwTaG9zdC5jb25zdWwuZXhhbXBsZTCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBANMM2XqJ1JJYTrydAMXHQ3Qk4jMEfbbJv+iBnMPSZudMVWPq
oORhYNUVq0Gjg6uxo6nfyQ1AMLUwZ0DLNcrep27B3tkIWw+F3qrWfun079WaRt5P
BXgfzNNcRaqL3M0Vf4nu+8HeL6qDw/gHHw/DQwBSbd4bcFIBiKpEBz99vUod07S4
GvWTArNTKE0/jzmBlDy/G8LfUegpTa7nGx+AbOjp19mRWyhe+TvThyjcVk5qQ0qH
bzhWxciqbNwX5z7hdGepo+TOcmXgo5rX0tuT67DN6TARiuWB29DJRX2SKyBm6DG6
8/RfwTECeuKTHuNysKh7052yloaej8y4RJDxi58CAwEAATANBgkqhkiG9w0BAQsF
AAOCAQEAHF10odRNJ7TKvcD2JPtR8wMacfldSiPcQnn+rhMUyBaKOoSrALxOev+N
L8N+RtEV+KXkyBkvT71OZzEpY9ROwqOQ/acnMdbfG0IBPbg3c/7WDD2sjcdr1zvc
U3T7WJ7G3guZ5aWCuAGgOyT6ZW8nrDa4yFbKZ1PCJkvUQ2ttO1lXmyGPM533Y2pi
SeXP6LL7z5VNqYO3oz5IJEstt10IKxdmb2gKFhHjgEmHN2gFL0jaPi4mjjaINrxq
MdqcM9IzLr26AjZ45NuI9BCcZWO1mraaQTOIb3QL5LyqaC7CRJXLYPSGARthyDhq
J3TrQE3YVrL4D9xnklT86WDnZKApJg==
-----END CERTIFICATE-----
EOF
'

upsert_config_entry primary '
kind = "inline-certificate"
name = "also-host-consul-example"
private_key = <<EOF
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0wzZeonUklhOvJ0AxcdDdCTiMwR9tsm/6IGcw9Jm50xVY+qg
5GFg1RWrQaODq7Gjqd/JDUAwtTBnQMs1yt6nbsHe2QhbD4XeqtZ+6fTv1ZpG3k8F
eB/M01xFqovczRV/ie77wd4vqoPD+AcfD8NDAFJt3htwUgGIqkQHP329Sh3TtLga
9ZMCs1MoTT+POYGUPL8bwt9R6ClNrucbH4Bs6OnX2ZFbKF75O9OHKNxWTmpDSodv
OFbFyKps3BfnPuF0Z6mj5M5yZeCjmtfS25PrsM3pMBGK5YHb0MlFfZIrIGboMbrz
9F/BMQJ64pMe43KwqHvTnbKWhp6PzLhEkPGLnwIDAQABAoIBADBEJAiONPszDu67
yU1yAM8zEDgysr127liyK7PtDnOfVXgAVMNmMcsJpZzhVF+TxKY487YAFCOb6kE7
OBYpTYla9SgVbR3js8TGQUgoKCFlowd8cvfB7gn4dEZIrjqIzB4zdYgk1Cne8JZs
qoHkWhJcx5ugEtPuXd7yp+WxT/T+6uOro06scp67NhP5t9yoAGFv5Vdb577RuzRo
Wkd9higQ9A20+GtjCY0EYxdgRviWvW7mM5/F+Lzcaui86ME+ga754gX8zgW3+NJ5
LMsz5OLSnh291Uyjmr77HWBv/xvpq01Fls0LyJcgxFVZuJs5GQz+l3otSqv4FTP6
Ua9w/YECgYEA8To3dgUK1QhzX5rwhWtlst3pItGTvmEdNzXmjgSylu7uKM13i+xg
llhp2uXrOEtuL+xtBZdeFNaijusbyqjg0xj6e4o31c19okuuDkJD5/sfQq22bvrn
gVJMGuESprIiPePrEyrXCHOdxH6eDgR2dIzAeO5vz0nnKGFAWrJJbvECgYEA3/mJ
eacXOJznw4Sa8jGWS2FtZLKxDHph7uDKMJmuG0ukb3aHJ9dMHrPleCLo8mhpoObA
hueoIbIP7swGrQx79+nZbnQpF6rMp6FAU5bF3gSrj1eWbaeh8pn9mrv4hal9USmn
orTbXMxDp3XSh7voR8Fqy5tMQqwZ+Lz74ccbw48CgYEA5cEhGdNrocPOv3x/IVRN
JLOfXX5nTaiJfxBja1imEIO5ajtoZWjaBdhn2gmqo4+UfyicHfsxrH9RjPX5HmkC
2Yys5gWbcJOr2Wxjd0k+DDFucL+rRsDKxq1vtxov/X0kh/YQ68ydynr0BTbjq04s
1I1KtOPEspYdCKS3+qpcrsECgYBtvYeVesBO9do9G0kMKC26y4bdEwzaz1ASykNn
IrWDHEH6dznr1HqwhHaHsZsvwucWdlmZAAKKWAOkfoU63uYS55qomvPTa9WQwNqS
2koi6Wjh+Al1uvAHvVncKgOwAgar8Nv5ReJBirgPYhSAexpppiRclL/93vNuw7Iq
wvMgkwKBgQC5wnb6SUUrzzKKSRgyusHM/XrjiKgVKq7lvFE9/iJkcw+BEXpjjbEe
RyD0a7PRtCfR39SMVrZp4KXVNNK5ln0WhuLvraMDwOpH9JDWHQiAhuJ3ooSwBylK
+QCLjyOtWAGZAIBRJyb1txfTXZ++dldkOjBi3bmEiadOa48ksvDsNQ==
-----END RSA PRIVATE KEY-----
EOF

certificate = <<EOF
-----BEGIN CERTIFICATE-----
MIIDQjCCAioCCQC6cMRYsE+ahDANBgkqhkiG9w0BAQsFADBjMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCQ0ExCzAJBgNVBAcMAkxBMQ0wCwYDVQQKDARUZXN0MQ0wCwYD
VQQLDARTdHViMRwwGgYDVQQDDBNob3N0LmNvbnN1bC5leGFtcGxlMB4XDTIzMDIx
NzAyMTA1MloXDTI4MDIxNjAyMTA1MlowYzELMAkGA1UEBhMCVVMxCzAJBgNVBAgM
AkNBMQswCQYDVQQHDAJMQTENMAsGA1UECgwEVGVzdDENMAsGA1UECwwEU3R1YjEc
MBoGA1UEAwwTaG9zdC5jb25zdWwuZXhhbXBsZTCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBANMM2XqJ1JJYTrydAMXHQ3Qk4jMEfbbJv+iBnMPSZudMVWPq
oORhYNUVq0Gjg6uxo6nfyQ1AMLUwZ0DLNcrep27B3tkIWw+F3qrWfun079WaRt5P
BXgfzNNcRaqL3M0Vf4nu+8HeL6qDw/gHHw/DQwBSbd4bcFIBiKpEBz99vUod07S4
GvWTArNTKE0/jzmBlDy/G8LfUegpTa7nGx+AbOjp19mRWyhe+TvThyjcVk5qQ0qH
bzhWxciqbNwX5z7hdGepo+TOcmXgo5rX0tuT67DN6TARiuWB29DJRX2SKyBm6DG6
8/RfwTECeuKTHuNysKh7052yloaej8y4RJDxi58CAwEAATANBgkqhkiG9w0BAQsF
AAOCAQEAHF10odRNJ7TKvcD2JPtR8wMacfldSiPcQnn+rhMUyBaKOoSrALxOev+N
L8N+RtEV+KXkyBkvT71OZzEpY9ROwqOQ/acnMdbfG0IBPbg3c/7WDD2sjcdr1zvc
U3T7WJ7G3guZ5aWCuAGgOyT6ZW8nrDa4yFbKZ1PCJkvUQ2ttO1lXmyGPM533Y2pi
SeXP6LL7z5VNqYO3oz5IJEstt10IKxdmb2gKFhHjgEmHN2gFL0jaPi4mjjaINrxq
MdqcM9IzLr26AjZ45NuI9BCcZWO1mraaQTOIb3QL5LyqaC7CRJXLYPSGARthyDhq
J3TrQE3YVrL4D9xnklT86WDnZKApJg==
-----END CERTIFICATE-----
EOF
'

upsert_config_entry primary '
kind = "inline-certificate"
name = "other-consul-example"
private_key = <<EOF
-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA7Qgf93OZYlpM9UvfbFBUeK+udCTfy+yRbTKYgti5qKyZQwnT
7GtxpNxWWNT7ecqGZJb3BxhIVjTYZwZYB1zDzHzE7zTVydrTbRP4MhaJhqWNdPdq
Ctth9QsX2r6KFPogvUPaYVrQBbXHoGTVcCZlFJOlEGGnuIU2cJcZEryxJKv3mB4P
FpHKP/YXy3qQb4A0+REGI93atGCxhCO5Mu5lgG+x54nKCdIXy3LVAyDqSjPuR01s
+ifdDkYUfvVY3DOb0bcNaMwi752GkfPuuxsKeHAsHmWHWOxkFiQAy3eSLJjSDghF
66JVZ6wx74Awb8NBm9nmYTbnZAUn3Z0tN6jlFwIDAQABAoIBAGh8EWNJ8M4bEht7
A5TCYEoG3zbRXlmNAZoKGJJtKIIC+1hCx8lKn4DVo7ZqxCOus8k5htD40kI170KS
2FD+gkzsnv724lqlfFdz2w9xQdQ5u/5YZcU9aZPT/QLuxP10OORVObl6h4JM3B+G
81MJibslTjjHY2CCUDoXUPUiek+4KTI6BYn9PL+ReCaH9qv85hIWvkgbwU2f9i3n
T8MysgDBNoKbVAqsH5Hq/zFkW5iad/zpgW+REgYK/gjBfUNEsRusm1GYim4XJtZf
SmXBtdYhk8kDBrMk1mNdZeMXMfl0VwZaHca8QmqrgDm//grUYKLWZO/PbDoI5c+w
/MTN6tkCgYEA956LZJ+Ts9g9k+jLcUV1hOAmR6cAL5tjEkBPVC5FC+5o42qOgI58
5NQjmVCM+mD9w+deAXkT/IDXzV28Y02lZ1pyOXTN+ChE2ZBym+yu2zyLj45lJzx5
h+lC7aeCBIGyjP/t6wFoFdCaluAsWpU4SY1DWtExcCLvXs0+zDiYjZMCgYEA9Q3Y
mlrAvdzb61x8BAEPpBTMSu0a+jo8mkJoUFS4ndFXEl5yPzqf6IUrFgzqgVruQg7k
dIZK5uDqNI7sXDXulpPFHbJdpAZq3Dlkv0VOvcgRycuWgDAyOngCSRN8JC7u4P5j
54xgAYQSCC/kPolFMHntXQLjzPevJNrkKnJfXO0CgYEAyCf5ByJSs0o1JE1Fvc7m
mrzRVJPye4kAQS2IskQgfe9+C24DuHj1DcdI61IIUw95sRRhkZE8jZvcVN3TPPXz
oKKkuDrpjxGF7dNsQQvFn+PF8AmrTFb+6dSszAvd9iScnor110OwzglsHE8iqyn5
cMLmUg/NBZbHpPsFKvEIp08CgYAmQ1Az4cHAo5CvMlSm52eCzkCL3nPc6GT4DTBu
gpwFAF/hHWAnYUcArnJo0gF3yzPympKvYxyk6i+Hn11mlIE5f79CgMxARURAOLHz
b6X42hl08dYBFAVzvbNVp7Y1jCJ+fRoqWG/RLMcIAjpYTWTBSfh3EnFxWqc9UPRZ
cFxVjQKBgQDUXDww+jXXTFlYkbhgNOtQS57M8jKWHnUxEc7IXOjF/0WGEEBAc2uc
fFW5dnPyS3D6vecWA8KB8LKcXey3ZcDZnVkahSgYhYHrGJrANzhfM3yli+lSFWsT
r295+0za3P3LVL4aKS+ok9oYZaVyMe/vlCMrIS+sh6SEc4+v0qNa3g==
-----END RSA PRIVATE KEY-----
EOF

certificate = <<EOF
-----BEGIN CERTIFICATE-----
MIIDQjCCAioCCQC2H6+PYz23xDANBgkqhkiG9w0BAQsFADBjMQswCQYDVQQGEwJV
UzELMAkGA1UECAwCQ0ExCzAJBgNVBAcMAkxBMQ0wCwYDVQQKDARUZXN0MQwwCgYD
VQQLDANGb28xHTAbBgNVBAMMFG90aGVyLmNvbnN1bC5leGFtcGxlMB4XDTIzMDIx
NzAyMTM0OVoXDTI4MDIxNjAyMTM0OVowYzELMAkGA1UEBhMCVVMxCzAJBgNVBAgM
AkNBMQswCQYDVQQHDAJMQTENMAsGA1UECgwEVGVzdDEMMAoGA1UECwwDRm9vMR0w
GwYDVQQDDBRvdGhlci5jb25zdWwuZXhhbXBsZTCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBAO0IH/dzmWJaTPVL32xQVHivrnQk38vskW0ymILYuaismUMJ
0+xrcaTcVljU+3nKhmSW9wcYSFY02GcGWAdcw8x8xO801cna020T+DIWiYaljXT3
agrbYfULF9q+ihT6IL1D2mFa0AW1x6Bk1XAmZRSTpRBhp7iFNnCXGRK8sSSr95ge
DxaRyj/2F8t6kG+ANPkRBiPd2rRgsYQjuTLuZYBvseeJygnSF8ty1QMg6koz7kdN
bPon3Q5GFH71WNwzm9G3DWjMIu+dhpHz7rsbCnhwLB5lh1jsZBYkAMt3kiyY0g4I
ReuiVWesMe+AMG/DQZvZ5mE252QFJ92dLTeo5RcCAwEAATANBgkqhkiG9w0BAQsF
AAOCAQEAijm6blixjl+pMRAj7EajoPjU+GqhooZayJrvdwvofwcPxQYpkPuh7Uc6
l2z494b75cRzMw7wS+iW/ad8NYrfw1JwHMsUfncxs5LDO5GsKl9Krg/39goDl3wC
ywTcl00y+FMYfldNPjKDLunENmn+yPa2pKuBVQ0yOKALp+oUeJFVzRNPV5fohlBi
HjypkO0KaVmCG6P01cqCgVkNzxnX9qQYP3YXX1yt5iOcI7QcoOa5WnRhOuD8WqJ1
v3AZGYNvKyXf9E5nD0y2Cmz6t1awjFjzMlXMx6AdHrjWqxtHhYQ1xz4P4NfzK27m
cCtURSzXMgcrSeZLepBfdICf+0/0+Q==
-----END CERTIFICATE-----
EOF
'

upsert_config_entry primary '
Kind      = "proxy-defaults"
Name      = "global"
Config {
  protocol = "http"
}
'

upsert_config_entry primary '
kind = "http-route"
name = "api-gateway-route-one"
rules = [
  {
    services = [
      {
        name = "s1"
      }
    ]
  }
]
parents = [
  {
    name = "api-gateway"
    sectionName = "listener-one"
  }
]
'

upsert_config_entry primary '
kind = "http-route"
name = "api-gateway-route-two"
rules = [
  {
    services = [
      {
        name = "s2"
      }
    ]
  }
]
parents = [
  {
    name = "api-gateway"
    sectionName = "listener-two"
  }
]
'

upsert_config_entry primary '
kind = "service-intentions"
name = "s1"
sources {
  name = "api-gateway"
  action = "allow"
}
'

upsert_config_entry primary '
kind = "service-intentions"
name = "s2"
sources {
  name = "api-gateway"
  action = "deny"
}
'

register_services primary

gen_envoy_bootstrap api-gateway 20000 primary true
gen_envoy_bootstrap s1 19000
gen_envoy_bootstrap s2 19001