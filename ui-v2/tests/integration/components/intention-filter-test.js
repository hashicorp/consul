import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('intention-filter', 'Integration | Component | intention filter', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{intention-filter}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'Search'
  );

  // // Template block usage:
  // this.render(hbs`
  //   {{#intention-filter}}
  //     template block text
  //   {{/intention-filter}}
  // `);

  // assert.equal(this.$().text().trim(), 'template block text');
});
