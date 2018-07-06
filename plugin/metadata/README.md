# metadata

## Name

*metadata* - enable a meta data collector.

## Description

By enabling *metadata* any plugin that implements [metadata.Provider
interface](https://godoc.org/github.com/coredns/coredns/plugin/metadata#Provider) will be called for
each DNS query, at beginning of the process for that query, in order to add it's own Metadata to
context.

The metadata collected will be available for all plugins, via the Context parameter
provided in the ServeDNS function. The package (code) documentation has examples on how to inspect
and retrieve metadata a plugin might be interested in.

TODO: write about naming of the keys (labels).
TODO: write about extracting and using

## Syntax

~~~
metadata [ZONES... ]
~~~

* **ZONES** zones metadata should be invoked for.

## Plugins

metadata.Provider interface needs to be implemented by each plugin willing to provide metadata
information for other plugins. It will be called by metadata and gather the information from all
plugins in context.

Note: this method should work quickly, because it is called for every request.

## Examples

There are currently no in tree plugins that write or use metadata.

## Also See

The [Provider interface](https://godoc.org/github.com/coredns/coredns/plugin/metadata#Provider) and
the [package level](https://godoc.org/github.com/coredns/coredns/plugin/metadata) documentation.
