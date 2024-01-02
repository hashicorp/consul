/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (clickable, is) {
  return function (obj = {}, scope = '') {
    if (scope !== '') {
      scope = scope + ' ';
    }
    return {
      ...obj,
      ...{
        submit: clickable(scope + '[type=submit]'),
        submitIsEnabled: is(':not(:disabled)', scope + '[type=submit]'),
      },
    };
  };
}
