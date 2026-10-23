/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

const transition = (from, to) => ({
  from: from && { name: from, params: {}, queryParams: {} },
  to: { name: to, params: {}, queryParams: {} },
});

module('Unit | Service | navigation-focus', function (hooks) {
  setupTest(hooks);

  test('it moves focus on regular route changes', function (assert) {
    const service = this.owner.lookup('service:navigation-focus');
    assert.true(service.routeChangeValidator(transition('dc.nodes.index', 'dc.nodes.show')));
  });

  test('it does not move focus when nothing changed', function (assert) {
    const service = this.owner.lookup('service:navigation-focus');
    assert.false(service.routeChangeValidator(transition('dc.nodes.show', 'dc.nodes.show')));
  });

  test('it keeps focus on the tab for a TabNav route change, once', function (assert) {
    const service = this.owner.lookup('service:navigation-focus');
    service.tabTransitionTo('dc.nodes.show.sessions');
    assert.false(
      service.routeChangeValidator(
        transition('dc.nodes.show.healthchecks', 'dc.nodes.show.sessions.index')
      ),
      'the tab route change keeps focus on the tab'
    );
    assert.true(
      service.routeChangeValidator(transition('dc.nodes.show.sessions', 'dc.nodes.index')),
      'the next route change moves focus again'
    );
  });

  test('it moves focus when a TabNav transition ends up elsewhere', function (assert) {
    const service = this.owner.lookup('service:navigation-focus');
    service.tabTransitionTo('dc.nodes.show.sessions');
    assert.true(service.routeChangeValidator(transition('dc.nodes.show', 'dc.nodes.index')));
  });
});
