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

    await render(hbs`<div>{{format-ipaddr '2001::85a3::8a2e:370:7334'}}</div>`);
    assert.dom(this.element).hasText('Invalid IP Address');
  });
});
