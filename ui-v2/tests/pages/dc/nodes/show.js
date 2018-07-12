export default function(visitable, deletable, clickable, attribute, collection, radiogroup) {
  return {
    visit: visitable('/:dc/nodes/:node'),
    tabs: radiogroup('tab', ['health-checks', 'services', 'round-trip-time', 'lock-sessions']),
    healthchecks: collection('[data-test-node-healthcheck]', {
      name: attribute('data-test-node-healthcheck'),
    }),
    services: collection('#services [data-test-tabular-row]', {
      id: attribute('data-test-service-id', '[data-test-service-id]'),
      name: attribute('data-test-service-name', '[data-test-service-name]'),
      port: attribute('data-test-service-port', '.port'),
    }),
    sessions: collection(
      '#lock-sessions [data-test-tabular-row]',
      deletable({
        TTL: attribute('data-test-session-ttl', '[data-test-session-ttl]'),
      })
    ),
  };
}
