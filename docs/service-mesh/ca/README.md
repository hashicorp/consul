# Certificate Authority (Connect CA)

The Certificate Authority subsystem manages a CA trust chain for issuing certificates to
services and client agents (via auto-encrypt and auto-config).

## Architecture

### High level overview

- we can start with the mind map
- high level explaination of what are the features that are involved in CA (mesh/connect, auto encrypt)
- add all the func that are involved in the CA operations
- relationship between the different certs


### CA and Certificate relationship

This diagram shows the relationship between the CA certificates in Consul primary and
secondary.

![CA relationship](./cert-relationship.svg)

<sup>[source](./cert-relationship.mmd)</sup>

TODO: describe the relationship

* what does it mean for the external root CA to be optional
  * it always exists , unless the Consul CA provider is used AND it has generated the CA
    root.
* relationship between Primary Root CA and Signing CA in the primary
  * sometimes its the same thing (Consul, and AWS providers)
  * sometimes it is different (Vault provider)
* client agent cert is used by auto-encrypt for Agent HTTP TLS (and client side of RPC
  TLS)
* leaf cert service is the cert used by a service in the mesh

### detailed call flow
- sequence diagram for leader election
- sequence diagram for leaf signing
- sequence diagram for CA cert rotation
