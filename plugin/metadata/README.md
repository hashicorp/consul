# metadata

## Name

*metadata* - enables a metadata collector.

## Description

By enabling *metadata* any plugin that implements [metadata.Provider
interface](https://godoc.org/github.com/coredns/coredns/plugin/metadata#Provider) will be called for
each DNS query, at the beginning of the process for that query, in order to add its own metadata to
context.

The metadata collected will be available for all plugins, via the Context parameter provided in the
ServeDNS function. The package (code) documentation has examples on how to inspect and retrieve
metadata a plugin might be interested in.

The metadata is added by setting a label with a value in the context. These labels should be named
`plugin/NAME`, where **NAME** is something descriptive. The only hard requirement the *metadata*
plugin enforces is that the labels contain a slash. See the documentation for
`metadata.SetValueFunc`.

The value stored is a string. The empty string signals "no metadata". See the documentation for
`metadata.ValueFunc` on how to retrieve this.

## Syntax

~~~
metadata [ZONES... ]
~~~

* **ZONES** zones metadata should be invoked for.

## Plugins

`metadata.Provider` interface needs to be implemented by each plugin willing to provide metadata
information for other plugins. It will be called by metadata and gather the information from all
plugins in context.

Note: this method should work quickly, because it is called for every request.

## Examples

The *rewrite* plugin uses meta data to rewrite requests.

## Also See

The [Provider interface](https://godoc.org/github.com/coredns/coredns/plugin/metadata#Provider) and
the [package level](https://godoc.org/github.com/coredns/coredns/plugin/metadata) documentation.
