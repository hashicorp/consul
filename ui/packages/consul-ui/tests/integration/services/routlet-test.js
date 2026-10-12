/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';
import { run } from '@ember/runloop';

module('Integration | Routlet', function (hooks) {
  setupTest(hooks);
  test('outletFor works', function (assert) {
    const routlet = this.owner.lookup('service:routlet');
    routlet.addOutlet('application', {
      name: 'application',
    });
    routlet.addRoute('dc', {});
    routlet.addOutlet('dc', {
      name: 'dc',
    });
    routlet.addRoute('dc.services', {});
    routlet.addOutlet('dc.services', {
      name: 'dc.services',
    });
    routlet.addRoute('dc.services.instances', {});

    let actual = routlet.outletFor('dc.services');
    let expected = 'dc';
    assert.strictEqual(actual.name, expected);

    actual = routlet.outletFor('dc');
    expected = 'application';
    assert.strictEqual(actual.name, expected);

    actual = routlet.outletFor('application');
    expected = undefined;
    assert.strictEqual(actual, expected);
  });

  test('removeOutlet keeps an outlet re-registered under the same name', function (assert) {
    const routlet = this.owner.lookup('service:routlet');
    const previous = { name: 'dc.moved', model: 'previous' };
    const next = { name: 'dc.moved', model: 'next' };
    routlet.addOutlet('dc.moved', previous);

    // An Outlet re-created in the same render (e.g. moved into another tab
    // panel) registers before the old one's deferred removal runs.
    run(() => {
      routlet.removeOutlet('dc.moved', previous);
      routlet.addOutlet('dc.moved', next);
    });
    assert.strictEqual(routlet.modelFor('dc.moved'), 'next', 'the new outlet is kept');

    run(() => {
      routlet.removeOutlet('dc.moved', next);
    });
    assert.strictEqual(
      routlet.modelFor('dc.moved'),
      undefined,
      'the outlet is removed when it is still the registered one'
    );
  });
});
