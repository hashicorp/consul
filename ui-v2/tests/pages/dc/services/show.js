export default function(visitable, attribute, collection, text, filter) {
  return {
    visit: visitable('/:dc/services/:service'),
    externalSource: attribute('data-test-external-source', 'h1 span'),
    instances: collection('#instances [data-test-tabular-row]', {
      address: text('[data-test-address]'),
    }),
    dashboardAnchor: {
      href: attribute('href', '[data-test-dashboard-anchor]'),
    },
    filter: filter,
  };
}
