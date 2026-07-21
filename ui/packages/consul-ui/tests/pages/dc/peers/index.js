/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import tabgroup from 'consul-ui/components/tab-nav/pageobject';

export default function (visitable, creatable, items, clickable) {
  return creatable({
    visit: visitable('/:dc/peers'),
    peers: items(),
    sort: {
      name: clickable('.consul-peer-list thead th:nth-child(1) button.hds-table__th-button--sort'),
    },
    tabs: tabgroup('tab', ['imported-services', 'exported-services', 'server-addresses']),
  });
}
