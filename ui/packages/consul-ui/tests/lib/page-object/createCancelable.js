/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (clickable, is) {
  return function (obj, scope = '') {
    if (scope !== '') {
      scope = scope + ' ';
    }
    return {
      ...obj,
      ...{
        cancel: clickable(scope + '[type=reset]'),
        cancelIsEnabled: is(':not(:disabled)', scope + '[type=reset]'),
      },
    };
  };
}
