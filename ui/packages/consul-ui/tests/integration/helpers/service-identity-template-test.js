import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Helper | service-identity-template', function (hooks) {
  setupRenderingTest(hooks);

  test('renders default output with only name', async function (assert) {
    this.set('name', 'api');
    await render(hbs`<pre>{{service-identity-template this.name}}</pre>`);

    assert.dom('pre').hasText(
      `service "api" {
        policy = "write"
      }
      service "api-sidecar-proxy" {
        policy = "write"
      }
      service_prefix "" {
        policy = "read"
      }
      node_prefix "" {
        policy = "read"
      }`
    );
  });

  test('renders with canUseNspaces = true', async function (assert) {
    this.setProperties({
      name: 'api',
      canUseNspaces: true,
    });

    await render(
      hbs`<pre>{{service-identity-template this.name canUseNspaces=this.canUseNspaces}}</pre>`
    );

    assert.dom('pre').includesText('namespace "default"');
    assert.dom('pre').includesText('service "api"');
    assert.dom('pre').includesText('service_prefix ""');
    assert.dom('pre').includesText('node_prefix ""');
  });

  test('renders with canUsePartitions = true only', async function (assert) {
    this.setProperties({
      name: 'api',
      canUsePartitions: true,
      partition: 'p1',
    });

    await render(hbs`
      <pre>
        {{service-identity-template this.name
          canUsePartitions=this.canUsePartitions
          partition=this.partition}}
      </pre>
    `);

    assert.dom('pre').includesText('partition "p1"');
    assert.dom('pre').includesText('service "api"');
    assert.dom('pre').doesNotIncludeText('namespace');
  });

  test('renders with both canUsePartitions and canUseNspaces', async function (assert) {
    this.setProperties({
      name: 'api',
      canUsePartitions: true,
      canUseNspaces: true,
      partition: 'prod',
      nspace: 'secure',
    });

    await render(hbs`
      <pre>
        {{service-identity-template this.name
          partition=this.partition
          nspace=this.nspace
          canUsePartitions=this.canUsePartitions
          canUseNspaces=this.canUseNspaces}}
      </pre>
    `);

    assert.dom('pre').includesText('partition "prod"');
    assert.dom('pre').includesText('namespace "secure"');
    assert.dom('pre').includesText('service "api-sidecar-proxy"');
  });

  test('handles null name gracefully', async function (assert) {
    this.set('name', null);
    await render(hbs`<pre>{{service-identity-template this.name}}</pre>`);

    assert.dom('pre').includesText('service ""');
    assert.dom('pre').includesText('sidecar-proxy');
  });
});
