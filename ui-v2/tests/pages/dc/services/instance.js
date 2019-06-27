export default function(visitable, attribute, collection, text, radiogroup) {
  return {
    visit: visitable('/:dc/services/:service/:node/:id'),
    externalSource: attribute('data-test-external-source', 'h1 span'),
    tabs: radiogroup('tab', ['service-checks', 'node-checks', 'upstreams', 'tags']),
    serviceChecks: collection('#service-checks [data-test-healthchecks] li', {}),
    nodeChecks: collection('#node-checks [data-test-healthchecks] li', {}),
    upstreams: collection('#upstreams [data-test-tabular-row]', {
      name: text('[data-test-destination-name]'),
      datacenter: text('[data-test-destination-datacenter]'),
      type: text('[data-test-destination-type]'),
      address: text('[data-test-local-bind-address]'),
    }),
    proxy: {
      type: attribute('data-test-proxy-type', '[data-test-proxy-type]'),
      destination: attribute('data-test-proxy-destination', '[data-test-proxy-destination]'),
    },
  };
}
