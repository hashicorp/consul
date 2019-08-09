# azure

## Name

*azure* - enables serving zone data from Microsoft Azure DNS service.

## Description

The azure plugin is useful for serving zones from Microsoft Azure DNS. The *azure* plugin supports
all the DNS records supported by Azure, viz. A, AAAA, CNAME, MX, NS, PTR, SOA, SRV, and TXT
record types.

## Syntax

~~~ txt
azure RESOURCE_GROUP:ZONE... {
    tenant TENANT_ID
    client CLIENT_ID
    secret CLIENT_SECRET
    subscription SUBSCRIPTION_ID
    environment ENVIRONMENT
    fallthrough [ZONES...]
}
~~~

*   **RESOURCE_GROUP:ZONE** is the resource group to which the hosted zones belongs on Azure,
    and  **ZONE** the zone that contains data.

*   **CLIENT_ID** and **CLIENT_SECRET** are the credentials for Azure, and `tenant` specifies the
    **TENANT_ID** to be used. **SUBSCRIPTION_ID** is the subscription ID. All of these are needed
    to access the data in Azure.

*  `environment` specifies the Azure **ENVIRONMENT**.

*   `fallthrough` If zone matches and no record can be generated, pass request to the next plugin.
    If **ZONES** is omitted, then fallthrough happens for all zones for which the plugin is
    authoritative.

## Examples

Enable the *azure* plugin with Azure credentials for the zone `example.org`:

~~~ txt
example.org {
    azure resource_group_foo:example.org {
      tenant 123abc-123abc-123abc-123abc
      client 123abc-123abc-123abc-234xyz
      subscription 123abc-123abc-123abc-563abc
      secret mysecret
    }
}
~~~

## Also See

The [Azure DNS Overview](https://docs.microsoft.com/en-us/azure/dns/dns-overview).
