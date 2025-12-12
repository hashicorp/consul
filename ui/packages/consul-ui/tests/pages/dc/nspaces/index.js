/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, creatable, nspaces, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/namespaces'),
    nspaces: nspaces(),
    sort: popoverSelect('[data-test-sort-control]'),
  });
}
