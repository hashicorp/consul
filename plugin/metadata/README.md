# metadata

## Name

*metadata* - enable a metadata collector.

## Description

By enabling *metadata* any plugin that implements [metadata.Provider interface](https://godoc.org/github.com/coredns/coredns/plugin/metadata#Provider) will be called for each DNS query, at being of the process for that query, in order to add it's own Metadata to context. The metadata collected will be available for all plugins handler, via the Context parameter provided in the ServeDNS function.
Metadata plugin is automatically adding the so-called default medatada (extracted from the query) to the context. Those default metadata are: {qname}, {qtype}, {client_ip}, {client_port}, {protocol}, {server_ip}, {server_port}


## Syntax

~~~
metadata [ZONES... ]
~~~

## Plugins

metadata.Provider interface needs to be implemented by each plugin willing to provide metadata information for other plugins. It will be called by metadata and gather the information from all plugins in context.
Note: this method should work quickly, because it is called for every request
from the metadata plugin.
If **ZONES** is specified then metadata add is limited by zones. Metadata is added to every context going through metadata.Provider if **ZONES** are not specified.


## Examples

Enable metadata for all requests. Rewrite uses one of the provided by default metadata variables.

~~~ corefile
. {
    metadata
    rewrite edns0 local set 0xffee {client_ip}
    forward . 8.8.8.8:53
}
~~~

Add metadata for all requests within `example.org.`. Rewrite uses one of provided by default metadata variables. Any other requests won't have metadata.

~~~ corefile
. {
    metadata example.org
    rewrite edns0 local set 0xffee {client_ip}
    forward . 8.8.8.8:53
}
~~~
