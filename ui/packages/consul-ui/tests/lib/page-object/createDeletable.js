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
        delete: clickable(scope + '[data-test-delete]', { at: 0 }),
        confirmDelete: clickable(scope + '[data-test-delete]', { at: 0 }),
        confirmInlineDelete: clickable(scope + 'button.type-delete', { at: 0 }),
      },
    };
  };
}
