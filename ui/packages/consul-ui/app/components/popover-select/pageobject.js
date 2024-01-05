/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (clickable, collection) =>
  (scope = '.popover-select') => {
    return {
      scope: scope,
      selected: clickable('button'),
      options: collection('li[role="none"]', {
        button: clickable('button'),
      }),
    };
  };
