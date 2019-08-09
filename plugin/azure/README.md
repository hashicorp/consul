# azure

## Name

*azure* - enables serving zone data from Microsoft Azure DNS service.

## Description

The azure plugin is useful for serving zones from Microsoft Azure DNS.
Thi *azure* plugin supports all the DNS records supported by Azure, viz. A, AAAA, CAA, CNAME, MX, NS, PTR, SOA, SRV, and TXT record types. For a non-existing resource record, zone's SOA response will returned.


## Syntax

~~~ txt
azure RESOURCE_GROUP:ZONE... {
    tenant TENANT_ID
    client CLIENT_ID
    secret CLIENT_SECRET
    subscription SUBSCRIPTION_ID
}
~~~

*   **`RESOURCE_GROUP`** The resource group to which the dns hosted zones belong on Azure

*   **`ZONE`** the zone that contains the resource record sets to be
    accessed.

*   `fallthrough` If zone matches and no record can be generated, pass request to the next plugin.
    If **ZONES** is omitted, then fallthrough happens for all zones for which the plugin is
    authoritative. If specific zones are listed (for example `in-addr.arpa` and `ip6.arpa`), then
    only queries for those zones will be subject to fallthrough.

*   `environment` the azure environment to use. Defaults to `AzurePublicCloud`. Possible values: `AzureChinaCloud`, `AzureGermanCloud`, `AzurePublicCloud`, `AzureUSGovernmentCloud`.

## Examples

Enable the *azure* plugin with Azure credentials:

~~~ txt
. {
    azure resource_group_foo:foo.com {
      tenant 123abc-123abc-123abc-123abc
      client 123abc-123abc-123abc-123abc
      secret 123abc-123abc-123abc-123abc
      subscription 123abc-123abc-123abc-123abc
    }
}
~~~

## Also See
- [Azure DNS Overview](https://docs.microsoft.com/en-us/azure/dns/dns-overview)
