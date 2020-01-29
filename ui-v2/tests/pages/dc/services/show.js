export default function(visitable, attribute, collection, text, filter, isVisible) {
  return {
    visit: visitable('/:dc/services/:service'),
    externalSource: isVisible('[data-test-external-source]', { multiple: true }),
    instances: collection('#instances [data-test-tabular-row]', {
      address: text('[data-test-address]'),
    }),
    dashboardAnchor: {
      href: attribute('href', '[data-test-dashboard-anchor]'),
    },
    filter: filter,
  };
}
