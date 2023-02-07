export default function (
  visitable,
  clickable,
  attribute,
  isPresent,
  collection,
  text,
  intentions,
  tabs
) {
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
    peer: text('[data-test-peer-info] [data-test-peer-name]'),
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
      externalSource: attribute('data-test-external-source', '[data-test-external-source]'),
      instance: clickable('a'),
      nodeChecks: text('[data-test-node-health-checks]'),
      nodeName: text('[data-test-node-name]'),
    }),
    intentionList: intentions(),
  };
  page.tabs.topologyTab = {
    defaultAllowNotice: {
      see: isPresent('[data-test-notice="default-allow"]'),
    },
    filteredByACLs: {
      see: isPresent('[data-test-notice="filtered-by-acls"]'),
    },
    wildcardIntention: {
      see: isPresent('[data-test-notice="wildcard-intention"]'),
    },
    notDefinedIntention: {
      see: isPresent('[data-test-notice="not-defined-intention"]'),
    },
    noDependencies: {
      see: isPresent('[data-test-notice="no-dependencies"]'),
    },
    aclsDisabled: {
      see: isPresent('[data-test-notice="acls-disabled"]'),
    },
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
  page.tabs.tagsTab = {
    tags: collection('.tag-list dd > span', {
      name: text(),
    }),
  };
  return page;
}
