/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, click } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | consul/partition/list/table', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    this.owner.register('helper:can', () => true);
    // Actions/edit links aren't under test here and pull in the real router
    // (which isn't booted in a rendering test); stub href-to so the row can
    // render without it.
    this.owner.register('helper:href-to', () => '#');
  });

  test('it drives sort from the shared @sort state instead of re-sorting locally', async function (assert) {
    // Deliberately not alphabetically ordered: this table must display
    // @items exactly as given (the data layer is the source of truth for
    // order in controlled mode) rather than re-sorting them itself.
    this.set('items', [{ Name: 'zeta' }, { Name: 'alpha' }, { Name: 'mu' }]);
    this.set('sort', {
      value: 'Name:desc',
      change: (event) => this.set('sort.value', event.target.selected),
    });
    this.set('ondelete', () => {});

    await render(hbs`
      <Consul::Partition::List::Table @items={{this.items}} @ondelete={{this.ondelete}} @sort={{this.sort}} />
    `);

    assert
      .dom('[data-test-partition]')
      .exists({ count: 3 }, 'renders all rows without dropping/re-sorting any');
    const names = [...this.element.querySelectorAll('[data-test-partition]')].map((el) =>
      el.textContent.trim()
    );
    assert.deepEqual(
      names,
      ['zeta', 'alpha', 'mu'],
      'rows are rendered in the order the data layer already sorted @items, not re-sorted locally'
    );
    assert
      .dom('.hds-table__th--sort')
      .hasAttribute('aria-sort', 'descending', 'the header reflects the shared @sort state');

    await click('.hds-table__th-button--sort');

    assert.strictEqual(
      this.sort.value,
      'Name:asc',
      'clicking the column header toggles the same shared @sort state the toolbar dropdown drives'
    );
  });
});
