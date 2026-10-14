/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import matchesRoute from 'consul-ui/utils/routing/matches-route';
import { module, test } from 'qunit';

module('Unit | Utility | routing/matches-route', function () {
  test('it matches the route itself and its child routes', function (assert) {
    assert.true(matchesRoute('dc.nodes.show.sessions', 'dc.nodes.show.sessions'));
    assert.true(matchesRoute('dc.nodes.show.sessions.index', 'dc.nodes.show.sessions'));
  });
  test('it does not match sibling routes sharing a prefix', function (assert) {
    assert.false(matchesRoute('dc.services.show.services', 'dc.services.show.service'));
    assert.false(matchesRoute('dc.nodes.show.healthchecks', 'dc.nodes.show.sessions'));
  });
  test('it matches legacy nspace-prefixed routes', function (assert) {
    assert.true(matchesRoute('nspace.dc.nodes.show.sessions', 'dc.nodes.show.sessions'));
    assert.false(matchesRoute('nspace.dc.nodes.show.metadata', 'dc.nodes.show.sessions'));
  });
  test('it does not match missing routes', function (assert) {
    assert.false(matchesRoute(undefined, 'dc.nodes.show'));
    assert.false(matchesRoute('dc.nodes.show', undefined));
  });
});
