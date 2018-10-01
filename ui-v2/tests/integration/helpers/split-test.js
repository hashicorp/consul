import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('split', 'helper:split', {
  integration: true,
});

// Replace this with your real tests.
test('it renders', function(assert) {
  this.set('inputValue', 'a,string,split,by,a,comma');

  this.render(hbs`{{split inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'a,string,split,by,a,comma'
  );
});
