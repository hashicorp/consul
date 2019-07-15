import { moduleForComponent, test } from 'ember-qunit';
import hbs from 'htmlbars-inline-precompile';

moduleForComponent('stats-card', 'Integration | Component | stats card', {
  integration: true,
});

test('it renders', function(assert) {
  // Set any properties with this.set('myProperty', 'value');
  // Handle any actions with this.on('myAction', function(val) { ... });

  // Template block usage:
  this.render(hbs`
    {{#stats-card}}
      {{#block-slot 'icon'}}icon{{/block-slot}}
      {{#block-slot 'mini-stat'}}mini-stat{{/block-slot}}
      {{#block-slot 'header'}}header{{/block-slot}}
      {{#block-slot 'body'}}body{{/block-slot}}
    {{/stats-card}}
  `);
  ['icon', 'mini-stat', 'header'].forEach(item => {
    assert.ok(
      this.$('header')
        .text()
        .trim()
        .indexOf(item) !== -1
    );
  });
  assert.ok(
    this.$()
      .text()
      .trim()
      .indexOf('body') !== -1
  );
});
