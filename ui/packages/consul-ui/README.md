# consul-ui

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->


- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Yarn Commands](#yarn-commands)
- [Running / Development](#running--development)
  - [Browser 'Environment' Variables](#browser-environment-variables)
  - [Browser 'Debug Utility' Functions](#browser-debug-utility-functions)
  - [Code Generators](#code-generators)
  - [Running Tests](#running-tests)
  - [Linting](#linting)
  - [Building](#building)
    - [Running Tests in Parallel](#running-tests-in-parallel)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Prerequisites

You will need the following things properly installed on your computer.

* [Git](https://git-scm.com/)
* [Node.js](https://nodejs.org/) (with npm)

* [yarn](https://yarnpkg.com)
* [Ember CLI](https://ember-cli.com/)
* [Google Chrome](https://google.com/chrome/)

## Installation

* `git clone https://github.com/hashicorp/consul.git` this repository
* `cd ui/packages/consul-ui`
* `make start` or `yarn && yarn start`


## Yarn Commands

Most used tooling scripts below primarily use `make` which will `yarn install`
and in turn call node package scripts.

List of available project commands.  `yarn run <command-name>`

| Command | Description |
| ------- | ----------- |
| build:staging | Builds the UI in staging mode (ready for PR preview site). |
| build:ci | Builds the UI for CI. |
| build | Builds the UI for production. |
| lint | Runs all lint commands. |
| lint:hbs | Lints `hbs` template files. |
| lint:js | Lints `js` files. |
| format | Runs all auto-formatters. |
| format:js | Auto-formats `js` files using Prettier. |
| format:sass | Auto-formats `scss` files using Prettier. |
| start | Runs the development app on a local server using the mock API. |
| start:consul | Runs the development app local server using a real consul instance as the backend. |
| start:staging | Runs the staging app local server. |
| test | Runs the ember tests in a headless browser. |
| test:view | Runs the ember tests in a non-headless browser. |
| test:oss | Runs only the OSS ember tests in a headless browser. |
| test:oss:view | Runs only the OSS ember tests in a non-headless browser. |
| test:coverage:view | Runs only the test specified for coverage in a non-headless browser. |
| test:node | Runs tests that can't be run in ember using node. |
| doc:toc | Automatically generates a table of contents for this README file. |

## Running / Development

The source code comes with a small development mode that runs enough of the consul API
as a set of mocks/fixtures to be able to run the UI without having to run
consul.

* `make start` or `yarn start` to start the ember app
* Visit your app at [http://localhost:4200](http://localhost:4200).

You can also run the UI against a normal Consul installation.

* `consul server -dev` to start consul listening on http://localhost:8500
* `make start-consul` to start the ember app proxying to `consul` (this will
respect the `CONSUL_HTTP_ADDR` environment variable to locate the Consul
installation.
* Visit your app at [http://localhost:4200](http://localhost:4200).

Example: 

```bash
CONSUL_HTTP_ADDR=http://10.0.0.1:8500 make start-consul
```


### Browser 'Environment' Variables

In order to configure different configurations of Consul you can use Web
Inspector in your browser to set various cookie which to emulate different
configurations whislt using the mock API.

For example: to enable ACLs, use Web Inspector to set a cookie as follows:

```bash
CONSUL_ACLS_ENABLE=1
```

This will enable the ACLs login page, to which you can login with any ACL
token/secret.

| Variable | Default Value | Description |
| -------- | ------------- | ----------- |
| `CONSUL_ACLS_ENABLE` | false | Enable/disable ACLs support. |
| `CONSUL_ACLS_LEGACY` | false | Enable/disable legacy ACLs support. |
| `CONSUL_NSPACES_ENABLE` | false | Enable/disable Namespace support. |
| `CONSUL_SSO_ENABLE` | false | Enable/disable SSO support. |
| `CONSUL_OIDC_PROVIDER_URL` | undefined | Provide a OIDC provider URL for SSO. |
| `CONSUL_LATENCY` | 0 | Add a latency to network requests (milliseconds) |
| `CONSUL_METRICS_POLL_INTERVAL` | 10000 | Change the interval between requests to the metrics provider (milliseconds) |
| `CONSUL_METRICS_PROXY_ENABLE` | false | Enable/disable the metrics proxy |
| `CONSUL_METRICS_PROVIDER` | | Set the metrics provider to use for the Topology Tab |
| `CONSUL_SERVICE_DASHBOARD_URL` | | Set the template URL to use for Service Dashboard links |
| `CONSUL_UI_CONFIG` | | Set the entire `ui_config` for the UI as JSON if you don't want to use the above singular variables |
| `CONSUL_SERVICE_COUNT` | (random) | Configure the number of services that the API returns. |
| `CONSUL_NODE_COUNT` | (random) | Configure the number of nodes that the API returns. |
| `CONSUL_KV_COUNT` | (random) | Configure the number of KV entires that the API returns. |
| `CONSUL_INTENTION_COUNT` | (random) | Configure the number of intentions that the API returns. |
| `CONSUL_POLICY_COUNT` | (random) | Configure the number of policies that the API returns. |
| `CONSUL_ROLE_COUNT` | (random) | Configure the number of roles that the API returns. |
| `CONSUL_NSPACE_COUNT` | (random) | Configure the number of namespaces that the API returns. |
| `CONSUL_UPSTREAM_COUNT` | (random) | Configure the number of upstreams that the API returns. |
| `CONSUL_EXPOSED_COUNT` | (random) | Configure the number of exposed paths that the API returns. |
| `CONSUL_CHECK_COUNT` | (random) | Configure the number of health checks that the API returns. |
| `CONSUL_OIDC_PROVIDER_COUNT` | (random) | Configure the number of OIDC providers that the API returns. |
| `DEBUG_ROUTES_ENDPOINT` | undefined | When using the window.Routes() debug
utility ([see utility functions](#browser-debug-utility-functions)), use a URL to pass the route DSL to. %s in the URL will be replaced
with the route DSL - http://url.com?routes=%s  |

See `./mock-api` for more details.

### Browser 'Debug Utility' Functions

We currently have one 'debug utility', that only exists during development (they
are removed from the production build using Embers `runInDebug`). You can call
these either straight from the console in WebInspector, or by using javascript
URLs i.e. `javascript:Routes()`

| Variable | Arguments | Description |
| -------- | --------- | ----------- |
| `Routes(url)` | url: The url to pass the DSL to, if left `undefined` just use a blank tab | Provides a way to easily print out Embers Route DSL for the application or to pass it straight to any third party utility such as ember-diagonal |
| `Scenario(str)` | str: 'Cookie formatted' string, if left `undefined` open a new tab with a lonk to the current Scenario | Provides a way to easily save and reload scenarios of configurations via URLs or bookmarklets |

### Code Generators

Many classes used in the UI can be generated with ember generators, try `ember help generate` for more details

### Running Tests

Tests use the mock api (see ./mock-api for details)

* `make test` or `yarn run test`
* `make test-view` or `yarn run test:view` to view the tests running in Chrome

OSS only tests can also be run using:

* `make test-oss` or `yarn run test:oss`
* `make test-oss-view` or `yarn run test:oss:view` to view the tests running in Chrome

### Linting

`make lint` currently runs linting on the majority of js files and hbs files (using `ember-template-lint`).

See `.eslintrc.js` and `.eslintignore` for specific configuration.

### Building

* `make build` builds the UI for production usage (env=production)
* `make build-ci` builds the UI for CI/test usage (env=test)

Static files are built into ./dist

#### Running Tests in Parallel

Alternatively, `ember-exam` can be used to split the tests across multiple browser instances for faster results. Most options are the same as `ember test`. To see a full list of options, run `ember exam --help`.

**Note:** The `EMBER_EXAM_PARALLEL` environment variable must be set to override the default `parallel` value of `1` browser instance in [testem.js](./testem.js).

To quickly run the tests across 4 parallel browser instances:
```sh
make test-parallel
```

To run manually:
```sh
$ EMBER_EXAM_PARALLEL=true ./node_modules/.bin/ember exam --split <num> --parallel
```

More ways to split tests can be found in the [ember-exam README.md](https://github.com/trentmwillis/ember-exam/blob/master/README.md).
