/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

import { create } from 'ember-cli-page-object';

import obj from 'consul-ui/components/consul/intention/permission/header/form/pageobject';

const permissionHeaderForm = create(obj());

module('Integration | Component | consul/intention/permission/header/form', function (hooks) {
  setupRenderingTest(hooks);

  test('when IsPresent is selected we only show validate the header name', async function (assert) {
    this.set('permissionHeaderForm', permissionHeaderForm);
    // Handle any actions with this.set('myAction', function(val) { ... });

    await render(hbs`
      <Consul::Intention::Permission::Header::Form

      as |api|>
        <Ref @target={{this.PermissionHeaderForm}} @name="api" @value={{api}} />
      </Consul::Intention::Permission::Header::Form>
    `);

    assert.ok(permissionHeaderForm.Name.present);
    assert.ok(permissionHeaderForm.Value.present);

    await permissionHeaderForm.HeaderType.click();
    await permissionHeaderForm.HeaderType.option.IsPresent.click();

    assert.notOk(
      permissionHeaderForm.Value.present,
      `Value isn't present when IsPresent is selected`
    );

    await permissionHeaderForm.Name.fillIn('header');

    assert.ok(permissionHeaderForm.api.isDirty);
  });
});
