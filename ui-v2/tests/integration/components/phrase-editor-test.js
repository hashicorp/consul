import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('phrase-editor', 'Integration | Component | phrase editor', {
  integration: true,
});

test('it renders a phrase', function(assert) {
  this.set('value', ['phrase']);
  this.render(hbs`{{phrase-editor value=value}}`);
  assert.notEqual(
    this.$()
      .text()
      .trim()
      .indexOf('phrase'),
    -1
  );
});
test('it calls onchange when a phrase is removed by clicking the phrase remove button and refocuses', function(assert) {
  assert.expect(3);
  this.set('value', ['phrase']);
  this.on('change', function(e) {
    assert.equal(e.target.value.length, 0);
  });
  this.render(hbs`{{phrase-editor value=value onchange=(action 'change')}}`);
  assert.notEqual(
    this.$()
      .text()
      .trim()
      .indexOf('phrase'),
    -1
  );
  const $input = this.$('input');
  const $button = this.$('button');
  $button.trigger('click');
  assert.equal(document.activeElement, $input.get(0));
});
test('it calls onchange when a phrase is added', function(assert) {
  assert.expect(1);
  this.on('change', function(e) {
    assert.equal(e.target.value.length, 2);
  });
  this.set('value', ['phrase']);
  this.render(hbs`{{phrase-editor value=value onchange=(action 'change')}}`);
  const $input = this.$('input');
  $input.get(0).value = 'phrase 2';
  $input.trigger('input');
  $input.trigger('search');
});
