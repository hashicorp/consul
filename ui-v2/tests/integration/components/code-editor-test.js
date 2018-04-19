import { moduleForComponent, skip } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('code-editor', 'Integration | Component | code editor', {
  integration: true,
});

skip('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{code-editor}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    '1' // this comes with some strange whitespace
  );

  // Template block usage:
  this.render(hbs`
    {{#code-editor}}{{/code-editor}}
  `);

  assert.equal(
    this.$()
      .text()
      .trim(),
    '1'
  );
});
