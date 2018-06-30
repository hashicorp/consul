# Go Generator Plugin

This directory contains a `gnostic` plugin that can be used to generate a Go client library and scaffolding for a Go server for an API with an OpenAPI description.

The plugin can be invoked like this:

	gnostic bookstore.json --go-generator-out=bookstore

`bookstore` is the name of a directory where the generated code will be written.
`bookstore` will also be the package name used for generated code.

By default, both client and server code will be generated. If the `gnostic-go-generator` binary is also linked from the names `gnostic-go-client` and `gnostic-go-server`, then only client or only server code can be generated as follows:

	gnostic bookstore.json --go-client-out=bookstore

	gnostic bookstore.json --go-server-out=bookstore

For example usage, see the [examples/v2.0/bookstore](examples/v2.0/bookstore) directory.