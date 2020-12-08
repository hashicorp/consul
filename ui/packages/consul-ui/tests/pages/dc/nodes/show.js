export default function(
  visitable,
  deletable,
  clickable,
  attribute,
  collection,
  tabs,
  text,
  healthChecks
) {
  return {
    visit: visitable('/:dc/nodes/:node'),
    tabs: tabs('tab', [
      'health-checks',
      'service-instances',
      'round-trip-time',
      'lock-sessions',
      'metadata',
    ]),
    healthChecks: healthChecks(),
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
    metadata: collection('#metadata [data-test-tabular-row]', {}),
  };
}
