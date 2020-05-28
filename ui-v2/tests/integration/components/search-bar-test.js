import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render } from '@ember/test-helpers';
import { hbs } from 'ember-cli-htmlbars';

module('Integration | Component | search-bar', function(hooks) {
  setupRenderingTest(hooks);

  test('it renders', async function(assert) {
    // Set any properties with this.set('myProperty', 'value');
    this.set('search', function(e) {});

    await render(hbs`<SearchBar @onsearch={{action search}}/>`);

    assert.equal(this.element.textContent.trim(), 'Search');

    // Template block usage:
    await render(hbs`
      <SearchBar @onsearch={{action search}}></SearchBar>
    `);

    assert.equal(this.element.textContent.trim(), 'Search');
  });
});
