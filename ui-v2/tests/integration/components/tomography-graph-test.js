import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('tomography-graph', 'Integration | Component | tomography graph', {
  integration: true
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{tomography-graph}}`);

  assert.ok(this.$().text().trim().indexOf('ms') !== -1);

  // Template block usage:
  this.render(hbs`
    {{#tomography-graph}}{{/tomography-graph}}
  `);

  assert.ok(this.$().text().trim().indexOf('ms') !== -1);
});
