export default function(visitable, clickable, text, attribute, collection, page, popoverSort) {
  const service = {
    name: text('[data-test-service-name]'),
    service: clickable('a'),
    externalSource: attribute('data-test-external-source', '[data-test-external-source]'),
    kind: attribute('data-test-kind', '[data-test-kind]'),
  };
  return {
    visit: visitable('/:dc/services'),
    services: collection('.consul-service-list > ul > li:not(:first-child)', service),
    dcs: collection('[data-test-datacenter-picker]', {
      name: clickable('a'),
    }),
    navigation: page.navigation,
    home: clickable('[data-test-home]'),
    sort: popoverSort,
  };
}
