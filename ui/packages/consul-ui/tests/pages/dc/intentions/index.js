/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (visitable, creatable, clickable, intentions, popoverSelect) {
  return {
    visit: visitable('/:dc/intentions'),
    intentionList: intentions(),
    sort: popoverSelect('[data-test-sort-control]'),
    ...creatable({}),
  };
}
