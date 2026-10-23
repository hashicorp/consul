/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { module, test } from 'qunit';
import { setupRenderingTest } from 'ember-qunit';
import { render, click } from '@ember/test-helpers';
import hbs from 'htmlbars-inline-precompile';

module('Integration | Component | consul/peer/list', function (hooks) {
  setupRenderingTest(hooks);

  hooks.beforeEach(function () {
    // `can` gates the row's delete/write actions (LinkTo/route-based
    // dropdown) that this test isn't exercising; stub it out so the row can
    // render without a real router.
    this.owner.register('helper:can', () => false);
  });

  test('it drives sort from the shared @sort state instead of re-sorting locally', async function (assert) {
    // Deliberately not alphabetically ordered: this table must display
    // @items exactly as given (the data layer is the source of truth for
    // order in controlled mode) rather than re-sorting them itself.
    this.set('items', [
      { Name: 'zeta', State: 'ACTIVE' },
      { Name: 'alpha', State: 'ACTIVE' },
      { Name: 'mu', State: 'ACTIVE' },
    ]);
    this.set('sort', {
      value: 'Name:desc',
      change: (event) => this.set('sort.value', event.target.selected),
    });
    this.set('ondelete', () => {});

    await render(hbs`
      <Consul::Peer::List @items={{this.items}} @ondelete={{this.ondelete}} @sort={{this.sort}} />
    `);

    const names = [...this.element.querySelectorAll('[data-test-peer]')].map((el) =>
      el.textContent.trim()
    );
    assert.deepEqual(
      names,
      ['zeta', 'alpha', 'mu'],
      'rows are rendered in the order the data layer already sorted @items, not re-sorted locally'
    );

    const [nameHeader, statusHeader] = this.element.querySelectorAll('.hds-table__th--sort');
    assert
      .dom(nameHeader)
      .hasAttribute('aria-sort', 'descending', 'the Name header reflects the shared @sort state');
    assert
      .dom(statusHeader)
      .hasAttribute('aria-sort', 'none', 'the Status header is not marked as sorted');

    await click(nameHeader.querySelector('.hds-table__th-button--sort'));
    assert.strictEqual(
      this.sort.value,
      'Name:asc',
      'clicking the Name column header toggles the same shared @sort state the toolbar dropdown drives'
    );

    await click(statusHeader.querySelector('.hds-table__th-button--sort'));
    assert.strictEqual(
      this.sort.value,
      'State:asc',
      'clicking the Status column header also drives the same shared @sort state (not an independent local one)'
    );
  });
});
