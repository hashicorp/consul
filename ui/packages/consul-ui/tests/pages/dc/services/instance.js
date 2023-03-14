/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (
  visitable,
  alias,
  attribute,
  present,
  collection,
  text,
  tabs,
  upstreams,
  healthChecks
) {
  const page = {
    visit: visitable('/:dc/services/:service/instances/:node/:id'),
    externalSource: attribute('data-test-external-source', '[data-test-external-source]', {
      scope: '.title',
    }),
    tabs: tabs('tab', ['health-checks', 'upstreams', 'exposed-paths', 'addresses', 'tags-&-meta']),
    checks: alias('healthChecks.item'),
    healthChecks: healthChecks(),
    upstreams: alias('upstreamInstances.item'),
    upstreamInstances: upstreams(),
    exposedPaths: collection('[data-test-proxy-exposed-paths] > tbody tr', {
      combinedAddress: text('[data-test-combined-address]'),
    }),
    addresses: collection('.consul-tagged-addresses [data-test-tabular-row]', {
      address: text('[data-test-address]'),
    }),
    metadata: collection('.metadata [data-test-tabular-row]', {}),
  };
  page.tabs.healthChecksTab = {
    criticalSerfNotice: present('[data-test-critical-serf-notice]'),
    healthChecks: healthChecks(),
  };
  return page;
}
