import { create, visitable } from 'ember-cli-page-object';

import radiogroup from 'consul-ui/tests/lib/page-object/radiogroup';
export default create({
  visit: visitable('/:dc/nodes/:node'),
  tabs: radiogroup('tab', ['health-checks', 'services', 'round-trip-time', 'lock-sessions']),
});
