import { module, skip } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { find } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | healthchecked resource', function(hooks) {
  setupRenderingTest(hooks);

  skip('it renders', function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    // Handle any actions with this.on('myAction', function(val) { ... });

    this.render(hbs`{{healthchecked-resource}}`);

    assert.ok(
      find('*')
        .textContent.trim()
        .indexOf('other passing checks') !== -1
    );

    // Template block usage:
    this.render(hbs`
      {{#healthchecked-resource}}{{/healthchecked-resource}}
    `);

    assert.ok(
      find('*')
        .textContent.trim()
        .indexOf('other passing checks') !== -1
    );
  });
});
