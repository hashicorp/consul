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

* `make start-api` or `yarn start:api` (this starts a Consul API double running
on http://localhost:3000)
* `make start` or `yarn start` to start the ember app that connects to the
above API double
* Visit your app at [http://localhost:4200](http://localhost:4200).
* Visit your tests at [http://localhost:4200/tests](http://localhost:4200/tests).


### Code Generators

Make use of the many generators for code, try `ember help generate` for more details

### Running Tests

You do not need to run `make start-api`/`yarn run start:api` to run the tests

* `make test` or `yarn run test`
* `make test-view` or `yarn run test:view` to view the tests running in Chrome
