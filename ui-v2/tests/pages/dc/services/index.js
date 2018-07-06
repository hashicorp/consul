export default function(visitable, clickable, attribute, collection, page, filter) {
  return {
    visit: visitable('/:dc/services'),
    services: collection('[data-test-service]', {
      name: attribute('data-test-service'),
      service: clickable('a'),
    }),
    dcs: collection('[data-test-datacenter-picker]'),
    navigation: page.navigation,
    filter: filter,
  };
}
