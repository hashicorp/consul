/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (visitable, submitable, isPresent) {
  return submitable({
    visit: visitable('/setting'),
    blockingQueries: isPresent('[data-test-blocking-queries]'),
  });
}
