/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';
import { setupTest } from 'ember-qunit';

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
    assert.equal(actual.name, expected);

    actual = routlet.outletFor('dc');
    expected = 'application';
    assert.equal(actual.name, expected);

    actual = routlet.outletFor('application');
    expected = undefined;
    assert.equal(actual, expected);
  });
});
