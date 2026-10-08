/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, click } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | consul/data-table', function (hooks) {
  setupRenderingTest(hooks);

  test('it clamps the current page when @items shrinks to fewer pages (e.g. filtering/searching/deleting)', async function (assert) {
    // One item per page, so it's easy to tell which page is showing.
    this.set('pageSizes', [1]);
    this.set('columns', [{ label: 'Name' }]);
    this.set('items', [{ label: 'a' }, { label: 'b' }, { label: 'c' }]);

    await render(hbs`
      <Consul::DataTable @items={{this.items}} @columns={{this.columns}} @pageSizes={{this.pageSizes}} @ariaLabel="Test">
        <:row as |item B|>
          <B.Td data-test-item>{{item.label}}</B.Td>
        </:row>
      </Consul::DataTable>
    `);

    assert.dom('[data-test-item]').hasText('a', 'starts on page 1');

    const pageButton = (label) =>
      [...this.element.querySelectorAll('.hds-pagination-nav__number')].find(
        (el) => el.textContent.trim().split(/\s+/).pop() === label
      );

    await click(pageButton('3'));
    assert.dom('[data-test-item]').hasText('c', 'navigated to page 3');

    // Simulate the caller's data layer narrowing @items down to a single
    // match (filtering, searching, or deleting a row) while the table is
    // still sitting on page 3, which no longer exists.
    this.set('items', [{ label: 'only-match' }]);

    assert
      .dom('[data-test-item]')
      .hasText(
        'only-match',
        'clamps back to the last valid page instead of showing no rows even though a match exists'
      );
    assert
      .dom('.hds-pagination-nav__number[aria-current="page"]')
      .exists({ count: 1 }, 'exactly one page number is shown, matching the clamped total');
    assert
      .dom('.hds-pagination-nav__number[aria-current="page"]')
      .includesText(
        '1',
        'the pagination control itself reflects the clamped page, not the stale one'
      );
  });
});
