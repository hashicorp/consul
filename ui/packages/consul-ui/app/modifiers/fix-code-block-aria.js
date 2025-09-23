/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';

export default modifier(function fixCodeBlockAria(element) {
  function fixAria() {
    // Fix HDS CodeBlock ARIA issue - add role to pre elements with aria-labelledby
    element.querySelectorAll('pre[aria-labelledby]:not([role])').forEach((pre) => {
      pre.setAttribute('role', 'region');
    });
  }

  setTimeout(fixAria, 100);
  new MutationObserver(fixAria).observe(element, { childList: true, subtree: true });
});
