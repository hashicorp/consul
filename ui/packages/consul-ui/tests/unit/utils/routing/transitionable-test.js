import transitionable from 'consul-ui/utils/routing/transitionable';
import { module, test } from 'qunit';

const makeRoute = function(name, params = {}, parent) {
  return {
    name: name,
    paramNames: Object.keys(params),
    params: params,
    parent: parent,
  };
};
module('Unit | Utility | routing/transitionable', function() {
  test('it walks up the route tree to resolve all the required parameters', function(assert) {
    const expected = ['dc.service.instance', 'dc-1', 'service-0', 'node-0', 'service-instance-0'];
    const dc = makeRoute('dc', { dc: 'dc-1' });
    const service = makeRoute('dc.service', { service: 'service-0' }, dc);
    const instance = makeRoute(
      'dc.service.instance',
      { node: 'node-0', id: 'service-instance-0' },
      service
    );
    const actual = transitionable(instance, {});
    assert.deepEqual(actual, expected);
  });
  test('it walks up the route tree to resolve all the required parameters whilst replacing specified params', function(assert) {
    const expected = [
      'dc.service.instance',
      'dc-1',
      'service-0',
      'different-node',
      'service-instance-0',
    ];
    const dc = makeRoute('dc', { dc: 'dc-1' });
    const service = makeRoute('dc.service', { service: 'service-0' }, dc);
    const instance = makeRoute(
      'dc.service.instance',
      { node: 'node-0', id: 'service-instance-0' },
      service
    );
    const actual = transitionable(instance, { node: 'different-node' });
    assert.deepEqual(actual, expected);
  });
});
