/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (visitable, creatable, nspaces, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/namespaces'),
    nspaces: nspaces(),
    sort: popoverSelect('[data-test-sort-control]'),
  });
}
