import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('code-editor', 'Integration | Component | code editor', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{code-editor}}`);

  // this test is just to prove it renders something without producing
  // an error. It renders the number 1, but seems to also render some sort of trailing space
  // so just check for presence of CodeMirror
  assert.equal(this.$().find('.CodeMirror').length, 1);

  // Template block usage:
  this.render(hbs`
    {{#code-editor}}{{/code-editor}}
  `);
  assert.equal(this.$().find('.CodeMirror').length, 1);
});
