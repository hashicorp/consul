/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, creatable, authMethods, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/acls/auth-methods'),
    authMethods: authMethods(),
    sort: popoverSelect('[data-test-sort-control]'),
  });
}
