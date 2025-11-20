/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (clickable, property) {
  return function (obj = {}, scope = '') {
    if (scope !== '') {
      scope = scope + ' ';
    }
    
    const disabledProp = property('disabled', scope + '[type=submit]');
    
    return {
      ...obj,
      submit: clickable(scope + '[type=submit]'),
      submitIsEnabled: {
        isDescriptor: true,
        get() {
          return !disabledProp.get.call(this);
        }
      },
    };
  };
}
