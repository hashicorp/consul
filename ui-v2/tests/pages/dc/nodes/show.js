import { create, visitable, collection, attribute } from 'ember-cli-page-object';

import radiogroup from 'consul-ui/tests/lib/page-object/radiogroup';
export default create({
  visit: visitable('/:dc/nodes/:node'),
  tabs: radiogroup('tab', ['health-checks', 'services', 'round-trip-time', 'lock-sessions']),
  healthchecks: collection('[data-test-node-healthcheck]', {
    name: attribute('data-test-node-healthcheck'),
  }),
  services: collection('#services [data-test-tabular-row]', {
    port: attribute('data-test-service-port', '.port'),
  }),
  sessions: collection('#lock-sessions [data-test-tabular-row]', {
    TTL: attribute('data-test-session-ttl', '[data-test-session-ttl]'),
  }),
});
