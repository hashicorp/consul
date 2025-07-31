import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Helper | node-identity-template', function (hooks) {
  setupRenderingTest(hooks);

  test('renders default output with minimal args', async function (assert) {
    this.set('name', 'node-1');
    await render(hbs`<pre>{{node-identity-template this.name}}</pre>`);

    assert
      .dom('pre')
      .hasText(
        `node "node-1" {\n  policy = "write"\n}\nservice_prefix "" {\n  policy = "read"\n}`,
        'outputs default node and service_prefix block'
      );
  });

  test('renders with canUseNspaces true', async function (assert) {
    this.setProperties({
      name: 'node-1',
      canUseNspaces: true,
    });

    await render(
      hbs`<pre>{{node-identity-template this.name canUseNspaces=this.canUseNspaces}}</pre>`
    );

    assert.dom('pre').includesText('namespace "default"');
    assert.dom('pre').includesText('node "node-1"');
    assert.dom('pre').includesText('service_prefix ""');
  });

  test('renders with canUsePartitions true and canUseNspaces false', async function (assert) {
    this.setProperties({
      name: 'node-1',
      canUsePartitions: true,
      canUseNspaces: false,
      partition: 'alpha',
    });

    await render(
      hbs`<pre>{{node-identity-template this.name partition=this.partition canUsePartitions=this.canUsePartitions canUseNspaces=this.canUseNspaces}}</pre>`
    );

    assert.dom('pre').includesText('partition "alpha"');
    assert.dom('pre').includesText('node "node-1"');
    assert.dom('pre').includesText('service_prefix ""');
  });

  test('renders full structure when both canUsePartitions and canUseNspaces are true', async function (assert) {
    this.setProperties({
      name: 'node-1',
      canUsePartitions: true,
      canUseNspaces: true,
      partition: 'beta',
    });

    await render(
      hbs`<pre>{{node-identity-template this.name partition=this.partition canUsePartitions=this.canUsePartitions canUseNspaces=this.canUseNspaces}}</pre>`
    );

    assert.dom('pre').includesText('partition "beta"');
    assert.dom('pre').includesText('namespace "default"');
    assert.dom('pre').includesText('node "node-1"');
    assert.dom('pre').includesText('service_prefix ""');
  });

  test('handles undefined name safely', async function (assert) {
    await render(hbs`<pre>{{node-identity-template null}}</pre>`);

    assert.dom('pre').includesText('node ""');
  });
});
