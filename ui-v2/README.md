# consul-ui


## Prerequisites

You will need the following things properly installed on your computer.

* [Git](https://git-scm.com/)
* [Node.js](https://nodejs.org/) (with npm)
* [yarn](https://yarnpkg.com)
* [Ember CLI](https://ember-cli.com/)
* [Google Chrome](https://google.com/chrome/)

## Installation

* `git clone https://github.com/hashicorp/consul.git` this repository
* `cd ui-v2`
* `yarn install`

## Running / Development

The source code comes with a small server that runs enough of the consul API
as a set of mocks/fixtures to be able to run the UI without having to run
consul.

* `make start-api` or `yarn start:api` (this starts a Consul API double running
on http://localhost:3000)
* `make start` or `yarn start` to start the ember app that connects to the
above API double
* Visit your app at [http://localhost:4200](http://localhost:4200).

To enable ACLs using the mock API, use Web Inspector to set a cookie as follows:

```
CONSUL_ACLS_ENABLE=1
```

This will enable the ACLs login page, to which you can login with any ACL
token/secret.

You can also use a number of other cookie key/values to set various things whilst
developing the UI, such as (but not limited to):

```
CONSUL_SERVICE_COUNT=1000
CONSUL_NODE_CODE=1000
// etc etc
```

See `./node_modules/@hashicorp/consul-api-double` for more details.


### Code Generators

Make use of the many generators for code, try `ember help generate` for more details

### Running Tests

Please note: You do not need to run `make start-api`/`yarn run start:api` to run the tests, but the same mock consul API is used.

* `make test` or `yarn run test`
* `make test-view` or `yarn run test:view` to view the tests running in Chrome

#### Running Tests in Parallel
Alternatively, `ember-exam` can be used to split the tests across multiple browser instances for faster results. Most options are the same as `ember test`. To see a full list of options, run `ember exam --help`.

**Note:** The `EMBER_EXAM_PARALLEL` environment variable must be set to override the default `parallel` value of `1` browser instance in [testem.js](./testem.js).

To quickly run the tests across 4 parallel browser instances:
```sh
yarn test-parallel
```

To run manually:
```sh
$ EMBER_EXAM_PARALLEL=true ember exam --split <num> --parallel
```

More ways to split tests can be found in the [ember-exam README.md](https://github.com/trentmwillis/ember-exam/blob/master/README.md).