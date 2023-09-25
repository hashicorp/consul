/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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
