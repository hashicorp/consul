/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import routeInfoEqual from 'consul-ui/utils/routing/route-info-equal';
import { module, test } from 'qunit';

const routeInfo = (name, params = {}, queryParams = {}, parent) => ({
  name,
  params,
  queryParams,
  parent,
});

module('Unit | Utility | routing/route-info-equal', function () {
  test('it is equal for the same route tree, params and query params', function (assert) {
    const a = routeInfo('dc.nodes.show', { name: 'node-0' }, { filter: 'a' }, routeInfo('dc'));
    const b = routeInfo('dc.nodes.show', { name: 'node-0' }, { filter: 'a' }, routeInfo('dc'));
    assert.true(routeInfoEqual(a, b));
  });
  test('it is not equal when the route, params or query params differ', function (assert) {
    const a = routeInfo('dc.nodes.show', { name: 'node-0' }, {}, routeInfo('dc'));
    assert.false(
      routeInfoEqual(a, routeInfo('dc.nodes.index', { name: 'node-0' }, {}, routeInfo('dc')))
    );
    assert.false(
      routeInfoEqual(a, routeInfo('dc.nodes.show', { name: 'node-1' }, {}, routeInfo('dc')))
    );
    assert.false(
      routeInfoEqual(
        a,
        routeInfo('dc.nodes.show', { name: 'node-0' }, { filter: 'a' }, routeInfo('dc'))
      )
    );
    assert.false(
      routeInfoEqual(
        a,
        routeInfo('dc.nodes.show', { name: 'node-0' }, {}, routeInfo('dc', { dc: 'dc2' }))
      )
    );
  });
  test('it is not equal when either side is missing', function (assert) {
    assert.false(routeInfoEqual(undefined, routeInfo('dc')));
    assert.false(routeInfoEqual(routeInfo('dc'), null));
  });
});
