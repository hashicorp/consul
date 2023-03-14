/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
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
