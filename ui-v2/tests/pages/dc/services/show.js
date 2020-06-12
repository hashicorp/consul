export default function(visitable, attribute, collection, text, intentions, filter, tabs) {
  const page = {
    visit: visitable('/:dc/services/:service'),
    externalSource: attribute('data-test-external-source', '[data-test-external-source]', {
      scope: '.title',
    }),
    dashboardAnchor: {
      href: attribute('href', '[data-test-dashboard-anchor]'),
    },
    tabs: tabs('tab', [
      'instances',
      'linked-services',
      'upstreams',
      'intentions',
      'routing',
      'tags',
    ]),
    filter: filter(),

    // TODO: These need to somehow move to subpages
    instances: collection('.consul-service-instance-list > ul > li:not(:first-child)', {
      address: text('[data-test-address]'),
    }),
    intentions: intentions(),
  };
  page.tabs.upstreamsTab = {
    services: collection('.consul-upstream-list > ul > li:not(:first-child)', {
      name: text('[data-test-service-name]'),
    }),
  };
  page.tabs.linkedServicesTab = {
    services: collection('.consul-service-list > ul > li:not(:first-child)', {
      name: text('[data-test-service-name]'),
    }),
  };
  return page;
}
