export default function(visitable, attribute, collection, text, tabs) {
  return {
    visit: visitable('/:dc/services/:service/instances/:node/:id'),
    externalSource: attribute('data-test-external-source', '[data-test-external-source]', {
      scope: '.title',
    }),
    tabs: tabs('tab', ['health-checks', 'proxy-info', 'addresses', 'tags-&-meta']),
    checks: collection('[data-test-checks] li'),
    upstreams: collection('[data-test-proxy-upstreams] > li', {
      name: text('[data-test-destination-name]'),
    }),
    exposedPaths: collection('[data-test-proxy-exposed-paths] > tbody tr', {
      combinedAddress: text('[data-test-combined-address]'),
    }),
    proxyChecks: collection('[data-test-proxy-checks] li'),
    addresses: collection('#addresses [data-test-tabular-row]', {
      address: text('[data-test-address]'),
    }),
    metadata: collection('.metadata [data-test-tabular-row]', {}),
  };
}
