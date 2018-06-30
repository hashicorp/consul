# OpenAPI Swift Generator Plugin

This directory contains a `gnostic` plugin that can be used to generate a Swift client library and scaffolding for a Swift server for an API with an OpenAPI description.

The plugin can be invoked like this:

	gnostic bookstore.json --swift-generator-out=Bookstore

Where `Bookstore` is the name of a directory where the generated code will be written.

Both client and server code will be generated.

For example usage, see the [examples/bookstore](examples/bookstore) directory.

HTTP services are provided by the Kitura library.