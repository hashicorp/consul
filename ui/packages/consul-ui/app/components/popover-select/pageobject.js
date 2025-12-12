/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (clickable, collection) =>
  (scope = '.popover-select') => {
    return {
      scope: scope,
      selected: clickable('button', { at: 0 }),
      options: collection('li[role="none"]', {
        button: clickable('button'),
      }),
    };
  };
