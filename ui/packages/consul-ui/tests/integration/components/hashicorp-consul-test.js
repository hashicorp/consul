import { module, skip, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';
import { render } from '@ember/test-helpers';

module('Integration | Component | hashicorp consul', function(hooks) {
  setupRenderingTest(hooks);

  skip('it renders', function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    this.render(hbs`{{hashicorp-consul}}`);

    assert.dom('*').hasText('');

    // Template block usage:
    this.render(hbs`
      {{#hashicorp-consul}}
      {{/hashicorp-consul}}
    `);

    assert.dom('*').hasText('template block text');
  });

  module('datacenter dropdown', function(hooks) {
    hooks.beforeEach(function() {
      const router = this.owner.lookup('router:main');
      // we need to patch router as it relies on our custom location
      // implementation which isn't initialized correctly in rendering tests
      router.reopen({
        get location() {
          return {
            hrefTo() {},
          };
        },
        get currentRouteName() {
          return 'testing';
        },
      });

      // patch router service as there is no active route and helpers we use
      // rely on this method.
      const routerService = this.owner.lookup('service:router');
      routerService.reopen({
        isActive() {
          return false;
        },
      });
    });

    test('it does not display a dropdown when only one dc is available', async function(assert) {
      const dcs = [
        {
          Name: 'dc-1',
        },
      ];
      this.set('dcs', dcs);
      this.set('dc', dcs[0]);

      await render(hbs`<HashicorpConsul @dcs={{this.dcs}} @dc={{this.dc}} />`);

      assert
        .dom('[data-test-datacenter-menu]')
        .doesNotExist('datacenter dropdown is not displayed in nav');

      assert.dom('[data-test-datacenter]').hasText('dc-1', 'Datecenter name is displayed in nav');
    });

    test('it does displays a dropdown when more than one dc is available', async function(assert) {
      const dcs = [
        {
          Name: 'dc-1',
        },
        {
          Name: 'dc-2',
        },
      ];
      this.set('dcs', dcs);
      this.set('dc', dcs[0]);

      await render(hbs`<HashicorpConsul @dcs={{this.dcs}} @dc={{this.dc}} />`);

      assert
        .dom('[data-test-datacenter]')
        .doesNotExist('we are displaying more than just the name of the first dc');

      assert.dom('[data-test-datacenter-menu]').exists('datacenter dropdown is displayed');
    });
  });
});
