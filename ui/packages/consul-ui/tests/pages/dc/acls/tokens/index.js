/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (visitable, creatable, text, tokens, popoverSelect) {
  return {
    visit: visitable('/:dc/acls/tokens'),
    update: text('[data-test-notification-update]'),
    tokens: tokens(),
    sort: popoverSelect('[data-test-sort-control]'),
    ...creatable(),
  };
}
