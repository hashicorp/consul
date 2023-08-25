/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (visitable, creatable, roles, popoverSelect) {
  return {
    visit: visitable('/:dc/acls/roles'),
    roles: roles(),
    sort: popoverSelect('[data-test-sort-control]'),
    ...creatable(),
  };
}
