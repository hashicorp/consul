---
layout: "docs"
page_title: "Connect - Certificate Management"
sidebar_current: "docs-connect-ca-aws"
description: |-
  Consul can be used with AWS Certificate Manager Private CA to manage and sign certificates.
---

# AWS Certificate Manager Private CA as a Connect CA

Consul can be used with [AWS Certificate Manager (ACM) Private Certificate
Authority
(CA)](https://aws.amazon.com/certificate-manager/private-certificate-authority/)
to manage and sign certificates.

-> This page documents the specifics of the AWS ACM Private CA provider.
Please read the [certificate management overview](/docs/connect/ca.html)
page first to understand how Consul manages certificates with configurable
CA providers.

## Requirements

The ACM Private CA Provider was added in Consul 1.7.0.

The ACM Private CA Provider needs to be authorized via IAM credentials to
perform operations. Every Consul server needs to be running in an environment
where a suitable IAM configuration is present.

The [standard AWS SDK credential
locations](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials)
are used, which means that suitable credentials and region configuration need to be present in one of the following:

 1. Environment variables
 1. Shared credentials file
 1. Via an EC2 instance role

The IAM credential provided must have permission for the following actions:

 - CreateCertificateAuthority - assuming an existing CA is not specified in `existing_arn`
 - DescribeCertificateAuthority
 - GetCertificate
 - IssueCertificate

## Configuration

The ACM Private CA provider is enabled by setting the `ca_provider` to
`"aws-pca"`. At this time there is only one, optional configuration value.

```hcl
connect {
    enabled = true
    ca_provider = "aws-pca"
    ca_config {
      existing_arn = "arn:aws:acm-pca:region:account:certificate-authority/12345678-1234-1234-123456789012"
    }
}
```

 ~> Note that suitable AWS IAM credentials are necessary for the provider to
 work, however these are not configured in the Consul config which is typically
 on disk and rely on the [standard AWS SDK configuration
 locations](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials).

The configuration options are listed below. Note, the
first key is the value used in API calls and the second key (after the `/`)
is used if you're adding configuring to the agent's configuration file.

  * `ExistingARN` / `existing_arn` (`string: <optional>`) - The Amazon Resource
    Name (ARN) of an existing private CA in your ACM account. If specified,
    Consul will attempt to use the existing CA to issue certificates.
      - In the primary datacenter this ARN **must identify a root CA**. See
        [limitations](#limitations). 
      - In a secondary datacenter, it must identify a subordinate CA signed by
        the same root used in the primary datacenter. If it is signed by another
        root, Consul will automatically create a new subordinate signed by the
        primary's root instead.

      The default behavior with no `ExistingArn` specified is for Consul to
      create a new root CA in the primary datacenter and a subordinate CA in
      each secondary DC.

## Limitations

ACM Private CA has several
[limits](https://docs.aws.amazon.com/acm-pca/latest/userguide/PcaLimits.html)
that restrict how fast certificates can be issued. This may impact how quickly
large clusters can rotate all issued certificates.

Currently, the ACM Private CA provider for Connect has some additional
limitations described below.

### Unable to Cross-sign Other CAs

It's not possible to cross-sign other CA provider's root certificates during a
migration. ACM Private CA is capable of doing that through a different work flow
but is not able to blindly cross-sign another root certificate without a CSR
being generated. Both Consul's built-in CA and Vault can do this and the current
workflow for managing CAs relies on it.

For now, the limitation means that once ACM Private CA is configured as the CA
provider, it is not possible to reconfigure a different CA provider, or rotate
the root CA key without potentially observing some transient connection
failures. See the section on [forced rotation without
cross-signing](/docs/connect/ca.html#forced-rotation-without-cross-signing) for
more details.

### Primary DC Must be a Root CA

Currently, if an existing ACM Private CA is used, the primary DC must use a Root
CA directly to issue certificates.

## Cost Planning

To help estimate costs, an example is provided below of the resources that would
be used.

~> This is intended to illustrate the behavior of the CA for cost planning
purposes. Please refer to the [pricing for ACM Private
CA](https://aws.amazon.com/certificate-manager/pricing/) for actual cost
information.

Assume the following Consul datacenters exist and are configured to use ACM
Private CA as their Connect CA with the default leaf certificate lifetime of
72 hours:

| Datacenter | Primary | CA Resource Created | Number of service instances |
| --- | --- | --- | --- | --- |
| dc1 | yes | 1 ROOT | 100 |
| dc2 | no | 1 SUBORDINATE | 50 |
| dc3 | no | 1 SUBORDINATE | 500 |

Leaf certificates are valid for 72 hours but are refreshed when
between 60% and 90% of their lifetime has elapsed. On average each certificate
will be reissued every 54 hours or roughly 13.3 times per month.

So monthly cost would be calculated as:

 - 3 ⨉ Monthly CA cost, plus
 - 8630 ⨉ Certificate Issue cost, made up of:
    - 100 ⨉ 13.3 = 1,330 certificates issued in dc1
    - 50 ⨉ 13.3 = 665 certificates issued in dc2
    - 500 ⨉ 13.3 = 6,650 certificates issued in dc3

The number of certificates issued could be reduced by increasing
[`leaf_cert_ttl`](/docs/agent/options.html#ca_leaf_cert_ttl) in the CA Provider
configuration if the longer lived credentials are an acceptable risk tradeoff
against the cost.
