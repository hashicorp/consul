/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import tabgroup from 'consul-ui/components/tab-nav/pageobject';

export default function (visitable, creatable, items, clickable, collection) {
  return creatable({
    visit: visitable('/:dc/peers'),
    peers: items(),
    sort: {
      // In-table column header click-to-sort (Peer name column).
      name: clickable('.consul-peer-list thead th:nth-child(1) button.hds-table__th-button--sort'),
      // Toolbar "A to Z / Z to A / Status" sort dropdown.
      selected: clickable('[data-test-sort-control] button', { at: 0 }),
      options: collection('[data-test-sort-option]', {
        resetScope: true,
        testContainer: 'html',
        button: clickable(),
      }),
    },
    tabs: tabgroup('tab', ['imported-services', 'exported-services', 'server-addresses']),
  });
}
