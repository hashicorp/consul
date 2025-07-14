/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Helper | format-ipaddr', function (hooks) {
  setupRenderingTest(hooks);

  test('it renders the given value', async function (assert) {
    this.set('inputValue', '192.168.1.1');

    await render(hbs`<div>{{format-ipaddr this.inputValue}}</div>`);

    assert.dom(this.element).hasText('192.168.1.1');

    // await render(hbs`<div>{{format-ipaddr '2001::85a3::8a2e:370:7334'}}</div>`);
    // assert.dom(this.element).doesNotHaveText('2001::85a3::8a2e:370:7334');
  });

  test('it should return an empty string for invalid IP addresses', async function (assert) {
    this.set('inputValue', '2001::85a3::8a2e:370:7334');

    await render(hbs`<div>{{format-ipaddr this.inputValue}}</div>`);

    assert.dom(this.element).hasText('');
  });

  test('it should return a collapsed IPv6 address', async function (assert) {
    this.set('inputValue', '2001:0db8:0000:0000:0000:ff00:0042:8329');

    await render(hbs`<div>{{format-ipaddr this.inputValue}}</div>`);

    assert.dom(this.element).hasText('[2001:db8::ff00:42:8329]');
  });
});
