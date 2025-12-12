/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, creatable, clickable, intentions, popoverSelect) {
  return creatable({
    visit: visitable('/:dc/intentions'),
    intentionList: intentions(),
    sort: popoverSelect('[data-test-sort-control]'),
  });
}
