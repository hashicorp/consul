/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, creatable, clickable, intentions, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/intentions'),
    intentionList: intentions(),
    sort: {
      action: clickable(
        '.consul-intention-list-table thead th:nth-child(2) button.hds-table__th-button--sort'
      ),
    },
  });
}
