/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default (clickable, confirmation) => (actions, scope) => {
  return actions.reduce(
    (prev, item) => {
      const itemScope = `[data-test-${item}-action]`;
      return {
        ...prev,
        [item]: clickable(`${itemScope} [role='menuitem']`),
        [`confirm${item.charAt(0).toUpperCase()}${item.substr(1)}`]: clickable(
          "#confirm-modal [data-test-id='confirm-action']",
          {
            resetScope: true,
            testContainer: 'body', // modal is rendered in the body
          }
        ),
      };
    },
    {
      actions: clickable('label'),
    }
  );
};
