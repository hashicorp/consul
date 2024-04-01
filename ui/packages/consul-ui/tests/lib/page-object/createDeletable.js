/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (clickable) {
  return function (obj = {}, scope = '') {
    if (scope !== '') {
      scope = scope + ' ';
    }
    return {
      ...obj,
      ...{
        delete: clickable(scope + '[data-test-delete]'),
        confirmDelete: clickable(scope + '[data-test-delete]'),
        confirmInlineDelete: clickable(scope + 'button.type-delete'),
      },
    };
  };
}
