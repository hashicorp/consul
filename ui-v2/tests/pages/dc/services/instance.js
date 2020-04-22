export default function(visitable, attribute, collection, text, tabs) {
  return {
    visit: visitable('/:dc/services/:service/instances/:node/:id'),
    externalSource: attribute('data-test-external-source', '[data-test-external-source]', {
      scope: '.title-bar',
    }),
    tabs: tabs('tab', [
      'health-checks',
      'addresses',
      'upstreams',
      'exposed-paths',
      'tags',
      'meta-data',
    ]),
    serviceChecks: collection('[data-test-service-checks] li', {
      exposed: attribute('data-test-exposed', '[data-test-exposed]'),
    }),
    nodeChecks: collection('[data-test-node-checks] li', {
      exposed: attribute('data-test-exposed', '[data-test-exposed]'),
    }),
    upstreams: collection('#upstreams [data-test-tabular-row]', {
      name: text('[data-test-destination-name]'),
      datacenter: text('[data-test-destination-datacenter]'),
      type: text('[data-test-destination-type]'),
      address: text('[data-test-local-bind-address]'),
    }),
    exposedPaths: collection('#exposed-paths [data-test-tabular-row]', {
      combinedAddress: text('[data-test-combined-address]'),
    }),
    addresses: collection('#addresses [data-test-tabular-row]', {
      address: text('[data-test-address]'),
    }),
    metaData: collection('#meta-data [data-test-tabular-row]', {}),
  };
}
