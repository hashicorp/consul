---
layout: "docs"
page_title: "Kubernetes Auth Method"
sidebar_current: "docs-acl-auth-methods-kubernetes"
description: |-
  The Kubernetes auth method type allows for a Kubernetes service account token to be used to authenticate to Consul. This method of authentication makes it easy to introduce a Consul token into a Kubernetes pod.
---

-> **1.5.0+:**  This guide only applies in Consul versions 1.5.0 and newer.

# Kubernetes Auth Method

The `kubernetes` auth method type allows for a Kubernetes service account token
to be used to authenticate to Consul. This method of authentication makes it
easy to introduce a Consul token into a Kubernetes pod.

This page assumes general knowledge of [Kubernetes](https://kubernetes.io/) and
the concepts described in the main [auth method
documentation](/docs/acl/acl-auth-methods.html).

## Config Parameters

The following auth method [`Config`](/api/acl/auth-methods.html#config)
parameters are required to properly configure an auth method of type
`kubernetes`:

- `Host` `(string: <required>)` - Must be a host string, a host:port pair, or a
  URL to the base of the Kubernetes API server. 

- `CACert` `(string: <required>)` - PEM encoded CA cert for use by the TLS
  client used to talk with the Kubernetes API. NOTE: Every line must end with a
  newline (`\n`).

- `ServiceAccountJWT` `(string: <required>)` - A Service Account Token
  ([JWT](https://jwt.io/ "JSON Web Token")) used by the Consul leader to
  validate application JWTs during login.

### Sample Config

```json
{
    ...other fields...
    "Config": {
        "Host": "https://192.0.2.42:8443",
        "CACert": "-----BEGIN CERTIFICATE-----\n...-----END CERTIFICATE-----\n",
        "ServiceAccountJWT": "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9..."
    }
}
```

## RBAC

The Kubernetes service account corresponding to the configured
[`ServiceAccountJWT`](/docs/acl/auth-methods/kubernetes.html#serviceaccountjwt)
needs to have access to two Kubernetes APIs:

- [**TokenReview**](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#create-tokenreview-v1-authentication-k8s-io)

       -> Kubernetes should be running with `--service-account-lookup`. This is
       defaulted to true in Kubernetes 1.7, but any versions prior should ensure
       the Kubernetes API server is started with this setting. 

- [**ServiceAccount**](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.11/#read-serviceaccount-v1-core)
  (`get`)

The following is an example
[RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
configuration snippet to grant the necessary permissions to a service account
named `consul-auth-method-example`:

```yaml
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: review-tokens
  namespace: default
subjects:
- kind: ServiceAccount
  name: consul-auth-method-example
  namespace: default
roleRef:
  kind: ClusterRole
  name: system:auth-delegator
  apiGroup: rbac.authorization.k8s.io
---
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: service-account-getter
  namespace: default
rules:
- apiGroups: [""]
  resources: ["serviceaccounts"]
  verbs: ["get"]
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: get-service-accounts
  namespace: default
subjects:
- kind: ServiceAccount
  name: consul-auth-method-example
  namespace: default
roleRef:
  kind: ClusterRole
  name: service-account-getter
  apiGroup: rbac.authorization.k8s.io
```

## Trusted Identity Attributes

The authentication step returns the following trusted identity attributes for 
use in binding rule selectors and bind name interpolation.

| Attributes                 | Supported Selector Operations      | Can be Interpolated |
| -------------------------- | ---------------------------------- | ------------------- |
| `serviceaccount.namespace` | Equal, Not Equal                   | yes                 |
| `serviceaccount.name`      | Equal, Not Equal                   | yes                 |
| `serviceaccount.uid`       | Equal, Not Equal                   | yes                 |

## Kubernetes Authentication Details

Initially the
[`ServiceAccountJWT`](/docs/acl/auth-methods/kubernetes.html#serviceaccountjwt)
given to the Consul leader uses the TokenReview API to validate the provided
JWT. The trusted attributes of `serviceaccount.namespace`,
`serviceaccount.name`, and `serviceaccount.uid` are populated directly from the
Service Account metadata.

The Consul leader makes an additional query, this time to the ServiceAccount
API to check for the existence of an annotation of
`consul.hashicorp.com/service-name` on the ServiceAccount object. If one is
found its value will override the trusted attribute of `serviceaccount.name`
for the purposes of evaluating any binding rules.

