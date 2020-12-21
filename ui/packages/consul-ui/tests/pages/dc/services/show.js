export default function(visitable, attribute, collection, text, intentions, tabs) {
  const page = {
    visit: visitable('/:dc/services/:service'),
    externalSource: attribute('data-test-external-source', '[data-test-external-source]', {
      scope: '.title',
    }),
    dashboardAnchor: {
      href: attribute('href', '[data-test-dashboard-anchor]'),
    },
    metricsAnchor: {
      href: attribute('href', '[data-test-metrics-anchor]'),
    },
    tabs: tabs('tab', [
      'topology',
      'instances',
      'linked-services',
      'upstreams',
      'intentions',
      'routing',
      'tags',
    ]),
    // TODO: These need to somehow move to subpages
    instances: collection('.consul-service-instance-list > ul > li:not(:first-child)', {
      address: text('[data-test-address]'),
    }),
    intentionList: intentions(),
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
