import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('service/external-source', 'helper:service/external-source', {
  integration: true,
});

// Replace this with your real tests.
test('it renders', function(assert) {
  this.set('inputValue', { Meta: { 'external-source': 'consul' } });

  this.render(hbs`{{service/external-source inputValue}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'consul'
  );
});
test('it renders prefixed', function(assert) {
  this.set('inputValue', { Meta: { 'external-source': 'consul' } });

  this.render(hbs`{{service/external-source inputValue prefix='external-source-'}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'external-source-consul'
  );
});
