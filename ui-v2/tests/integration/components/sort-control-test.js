import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('sort-control', 'Integration | Component | sort control', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{sort-control}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    ''
  );

  // Template block usage:
  this.render(hbs`
    {{#sort-control}}
      template block text
    {{/sort-control}}
  `);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'template block text'
  );
});
test('it changes direction and calls onchange when clicked/activated', function(assert) {
  assert.expect(2);
  let count = 0;
  this.on('change', e => {
    if (count === 0) {
      assert.equal(e.target.value, 'sort:desc');
    } else {
      assert.equal(e.target.value, 'sort:asc');
    }
    count++;
  });
  this.render(hbs`{{sort-control checked=true value='sort' onchange=(action 'change')}}`);
  const $label = this.$('label');
  $label.trigger('click');
  $label.trigger('click');
});
