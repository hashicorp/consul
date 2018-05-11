import { moduleForComponent, skip } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('healthchecked-resource', 'Integration | Component | healthchecked resource', {
  integration: true,
});

skip('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{healthchecked-resource}}`);

  assert.ok(
    this.$()
      .text()
      .trim()
      .indexOf('other passing checks') !== -1
  );

  // Template block usage:
  this.render(hbs`
    {{#healthchecked-resource}}{{/healthchecked-resource}}
  `);

  assert.ok(
    this.$()
      .text()
      .trim()
      .indexOf('other passing checks') !== -1
  );
});
