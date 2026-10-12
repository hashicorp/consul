/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';

export default modifier(function fixModalBodyRole(element) {
  function fixRole() {
    // HDS's dialog-primitive body (rendered by <Hds::Modal>'s <M.Body>, and
    // reused by e.g. Hds::SlidingPanel) unconditionally renders
    // <div class="hds-dialog-primitive__body ..." tabindex="0">, regardless
    // of whether the body's content actually overflows and needs keyboard
    // scrolling. A tabbable element with no ARIA role fails "Name, Role,
    // Value" (the "generic" role a bare <div> gets isn't a valid *widget*
    // role for something reachable by Tab).
    //
    // The obvious fix — give it role="region" when it's actually scrollable
    // — turns out to trade one violation for another: IBM Equal Access
    // separately requires any *tabbable* element's role to be a widget role
    // specifically, and "region" is a landmark role, not a widget role (see
    // Hds::CodeBlock's identical situation, fixed the same way in
    // fix-code-block-aria.js). There's no ARIA widget role that fits a
    // passive scrollable content block, so — matching IBM's own first
    // suggested remedy — the body is always taken out of the tab order
    // instead, whether or not it happens to overflow.
    element.querySelectorAll('.hds-dialog-primitive__body[tabindex="0"]').forEach((body) => {
      body.setAttribute('tabindex', '-1');
    });
  }

  setTimeout(fixRole, 100);
  const observer = new MutationObserver(fixRole);
  observer.observe(element, { childList: true, subtree: true });

  return () => observer.disconnect();
});
