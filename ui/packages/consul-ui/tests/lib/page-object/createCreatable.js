/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (clickable, property) {
  return function (obj) {
    return {
      ...obj,
      ...{
        create: clickable('[data-test-create]'),
        createIsEnabled: property(':not(:disabled)', '[data-test-create]'),
      },
    };
  };
}
