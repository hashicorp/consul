/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';

export default modifier(function fixCodeBlockAria(element) {
  function fixAria() {
    // HDS's <Hds::CodeBlock> always renders its <pre> with tabindex="0" (so
    // long code is keyboard-scrollable) but, in this HDS version, no role —
    // a tabbable element with no role fails "Name, Role, Value". Giving it
    // role="region" (this modifier's original fix) satisfies that, but
    // creates a different violation: IBM Equal Access separately requires
    // any *tabbable* element's role to be a widget role specifically, and
    // "region" is a landmark role, not a widget role, so region + tabindex
    // ="0" together still fails. There's no ARIA widget role that fits
    // read-only scrollable text, so — matching IBM's own first suggested
    // remedy — take it out of the tab order instead of forcing a role that
    // doesn't hold up. `role="region"` is still added: it's fine (useful,
    // even) on a non-tabbable element, just not on a tabbable one.
    element.querySelectorAll('pre[aria-labelledby]:not([role])').forEach((pre) => {
      pre.setAttribute('role', 'region');
    });
    element.querySelectorAll('pre.hds-code-block__code[tabindex="0"]').forEach((pre) => {
      pre.setAttribute('tabindex', '-1');
    });
  }

  setTimeout(fixAria, 100);
  new MutationObserver(fixAria).observe(element, { childList: true, subtree: true });
});
