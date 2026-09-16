/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, creatable, clickable, intentions, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/intentions'),
    intentionList: intentions(),
    sort: {
      // "Intention type" (column 2) is no longer sortable per the migration
      // designs — only Source (1) and Permissions (4) keep a sort button.
      source: clickable(
        '.consul-intention-list-table thead th:nth-child(1) button.hds-table__th-button--sort'
      ),
      permissions: clickable(
        '.consul-intention-list-table thead th:nth-child(4) button.hds-table__th-button--sort'
      ),
    },
  });
}
