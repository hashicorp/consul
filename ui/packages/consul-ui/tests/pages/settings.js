/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (visitable, submitable, isPresent) {
  return submitable({
    visit: visitable('/setting'),
    blockingQueries: isPresent('[data-test-blocking-queries]'),
  });
}
