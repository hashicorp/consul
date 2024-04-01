/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (
  visitable,
  deletable,
  clickable,
  alias,
  attribute,
  present,
  collection,
  tabs,
  text,
  healthChecks
) {
  const page = {
    visit: visitable('/:dc/nodes/:node'),
    tabs: tabs('tab', [
      'health-checks',
      'service-instances',
      'round-trip-time',
      'lock-sessions',
      'metadata',
    ]),
    healthChecks: alias('tabs.healthChecksTab.healthChecks'),
    services: collection('.consul-service-instance-list > ul > li:not(:first-child)', {
      name: text('[data-test-service-name]'),
      port: attribute('data-test-service-port', '[data-test-service-port]'),
      externalSource: attribute('data-test-external-source', '[data-test-external-source]'),
    }),
    sessions: collection('.consul-lock-session-list [data-test-list-row]', {
      TTL: attribute('data-test-session-ttl', '[data-test-session-ttl]'),
      delay: text('[data-test-session-delay]'),
      actions: clickable('label'),
      ...deletable(),
    }),
    metadata: collection('.consul-metadata-list [data-test-tabular-row]', {}),
  };
  page.tabs.healthChecksTab = {
    criticalSerfNotice: present('[data-test-critical-serf-notice]'),
    healthChecks: healthChecks(),
  };
  return page;
}
