## Consul Web UI

This directory contains the Consul Web UI. Consul contains a built-in
HTTP server that serves this directoy, but any common HTTP server
is capable of serving it.

It uses JavaScript and [Ember](http://emberjs.com) to communicate with
the [Consul API](http://www.consul.io/docs/agent/http.html). The basic
features it provides are:

- Service view. A list of your registered services, their
health and the nodes they run on.
- Node view. A list of your registered nodes, the services running
on each and the health of the node.
- Key/value view and update

It's aware of multiple data centers, so you can get a quick global
overview before drilling into specific data-centers for detailed
views.

The UI uses some internal undocumented HTTP APIs to optimize
performance and usability.

### Development

Improvements and bug fixes are welcome and encouraged for the Web UI.

You'll need sass to compile CSS stylesheets. Install that with
bundler:

    cd ui/
    bundle

Reloading compilation for development:

    make watch

Consul ships with an HTTP server for the API and UI. By default, when
you run the agent, it is off. However, if you pass a `-ui-dir` flag
with a path to this directoy, you'll be able to access the UI via the
Consul HTTP server address, which defaults to `localhost:8500/ui`.

An example of this command, from inside the `ui/` directory, would be:

    consul agent -bootstrap -server -data-dir /tmp/ -ui-dir .


### Releasing

These steps are slightly manual at the moment.

1. Build with `make dist`
2. `dist/index.html`, replace the JS script src files between `<!-- ASSETS -->` tags with a single
tag linking to `src="static/application.min.js"`.

