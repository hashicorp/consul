/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (visitable, creatable, authMethods, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/acls/auth-methods'),
    authMethods: authMethods(),
    sort: popoverSelect('[data-test-sort-control]'),
  });
}
