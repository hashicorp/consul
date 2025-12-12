/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, creatable, policies, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/acls/policies'),
    policies: policies(),
    sort: popoverSelect('[data-test-sort-control]'),
  });
}
