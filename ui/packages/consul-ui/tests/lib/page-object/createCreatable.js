/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (clickable, is) {
  return function (obj) {
    return {
      ...obj,
      ...{
        create: clickable('[data-test-create]'),
        createIsEnabled: is(':not(:disabled)', '[data-test-create]'),
      },
    };
  };
}
