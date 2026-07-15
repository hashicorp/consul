/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, text, clickable, attribute, collection) {
  const node = {
    name: text('[data-test-node]'),
    leader: attribute('data-test-leader', '[data-test-leader]'),
    node: clickable('a', { at: 0 }),
    status: attribute('data-test-status', '[data-test-status]'),
  };
  return {
    visit: visitable('/:dc/nodes'),
    nodes: collection('.consul-node-table tbody tr', node),
    home: clickable('[data-test-home]', { at: 0 }),
    sort: {
      name: clickable('.consul-node-table thead th:nth-child(1) button.hds-table__th-button--sort'),
      health: clickable(
        '.consul-node-table thead th:nth-child(2) button.hds-table__th-button--sort'
      ),
    },
  };
}
