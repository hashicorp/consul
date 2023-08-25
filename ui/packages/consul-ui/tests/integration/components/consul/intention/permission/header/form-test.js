/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

import { create } from 'ember-cli-page-object';

import obj from 'consul-ui/components/consul/intention/permission/header/form/pageobject';

const PermissionHeaderForm = create(obj());

module('Integration | Component | consul/intention/permission/header/form', function (hooks) {
  setupRenderingTest(hooks);

  test('when IsPresent is selected we only show validate the header name', async function (assert) {
    this.set('PermissionHeaderForm', PermissionHeaderForm);
    // Handle any actions with this.set('myAction', function(val) { ... });

    await render(hbs`
      <Consul::Intention::Permission::Header::Form

      as |api|>
        <Ref @target={{PermissionHeaderForm}} @name="api" @value={{api}} />
      </Consul::Intention::Permission::Header::Form>
    `);

    assert.ok(PermissionHeaderForm.Name.present);
    assert.ok(PermissionHeaderForm.Value.present);

    await PermissionHeaderForm.HeaderType.click();
    await PermissionHeaderForm.HeaderType.option.IsPresent.click();

    assert.notOk(
      PermissionHeaderForm.Value.present,
      `Value isn't present when IsPresent is selected`
    );

    await PermissionHeaderForm.Name.fillIn('header');

    assert.ok(PermissionHeaderForm.api.isDirty);
  });
});
