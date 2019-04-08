# federation

## Name

*federation* - enables federated queries to be resolved via the kubernetes plugin.

## Description

Enabling this plugin allows
[Federated](https://kubernetes.io/docs/tasks/federation/federation-service-discovery/) queries to be
resolved via the kubernetes plugin.

Enabling *federation* without also having *kubernetes* is a noop.

## Syntax

~~~
federation [ZONES...] {
    NAME DOMAIN
    upstream
}
~~~

* Each **NAME** and **DOMAIN** defines federation membership. One entry for each. A duplicate
  **NAME** will silently overwrite any previous value.
* `upstream` resolve the `CNAME` target produced by this plugin.  CoreDNS
  will resolve External Services against itself and needs the *forward* plugin to be active to do
  so.

## Examples

Here we handle all service requests in the `prod` and `stage` federations.

~~~
. {
    kubernetes cluster.local
    federation cluster.local {
        prod prod.feddomain.com
        staging staging.feddomain.com
        upstream
    }
    forward . 192.168.1.12
}
~~~
