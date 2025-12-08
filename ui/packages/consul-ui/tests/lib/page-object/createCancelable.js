/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (clickable, property) {
  return function (obj, scope = '') {
    if (scope !== '') {
      scope = scope + ' ';
    }
    return {
      ...obj,
      ...{
        cancel: clickable(scope + '[type=reset]'),
        cancelIsEnabled: property(':not(:disabled)', scope + '[type=reset]'),
      },
    };
  };
}
