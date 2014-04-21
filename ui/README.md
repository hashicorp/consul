## Consul Web UI

This directory contains the Consul Web UI. Consul contains a built-in
HTTP server that serves this directoy, but any common HTTP server
is capable of serving it.

It uses JavaScript to communicate with the [Consul API](). The basic
features it provides are:

- Service view. A list of your registered services, their
health and the nodes they run on.
- Node view. A list of your registered nodes, the services running
on each and the health of the node.
- Key/value view and update

It's aware of multiple data centers, so you can get a quick global
overview before drilling into specific data-centers for detailed
views.

### Development

Improvements and bug fixes are welcome and encouraged for the Web UI.

The UI is built with SASS CSS, so you'll need to compile that through
the associated makefile, as well as installing the `sass` gem.

    gem install sass

One-time stylesheet compilation:

    make build

Reloading compilation for development:

    make watch

Additionally, you'll need to run a local webserver.

    make server


### Running the tests
