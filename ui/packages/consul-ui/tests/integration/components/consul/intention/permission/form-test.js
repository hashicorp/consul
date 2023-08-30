/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

import { create } from 'ember-cli-page-object';
import obj from 'consul-ui/components/consul/intention/permission/form/pageobject';

const PermissionForm = create(obj());
module('Integration | Component | consul/intention/permission/form', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function (assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.set('myAction', function(val) { ... });

    await render(hbs`
      <Consul::Intention::Permission::Form

      as |api|>

      </Consul::Intention::Permission::Form>
    `);

    await PermissionForm.Action.option.Deny.click();
    await PermissionForm.PathType.click();
    await PermissionForm.PathType.option.PrefixedBy.click();
    assert.ok(PermissionForm.Path.present);
    await PermissionForm.Path.fillIn('/path');
    await PermissionForm.AllMethods.click();
  });
});
