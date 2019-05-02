import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('phrase-editor', 'Integration | Component | phrase editor', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  this.render(hbs`{{phrase-editor}}`);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'Search'
  );

  // Template block usage:
  this.render(hbs`
    {{#phrase-editor}}
      template block text
    {{/phrase-editor}}
  `);

  assert.equal(
    this.$()
      .text()
      .trim(),
    'Search'
  );
});
