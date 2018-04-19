import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('consul-collection', 'Integration | Component | tabular collection', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{tabular-collection cell-layout=(fixed-grid-layout 800 50)}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    ''
  );

  // Template block usage:
  this.render(hbs`
    {{#tabular-collection cell-layout=(fixed-grid-layout 800 50)}}{{/tabular-collection}}
  `);

  assert.equal(
    this.$()
      .text()
      .trim(),
    ''
  );
});
