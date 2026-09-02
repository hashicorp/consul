/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';

const POPUP_ROLES = ['listbox', 'grid', 'tree', 'dialog'];

export default modifier(function fixSuperSelectAria(element) {
  function fixAria() {
    // Fix role="alert" → role="option" on select options
    element.querySelectorAll('[role="alert"][aria-selected]').forEach((el) => {
      el.setAttribute('role', 'option');
    });

    // Give the popup a combobox's aria-controls/aria-owns points at a valid
    // role. ember-power-select wraps its ARIA listbox in an outer
    // positioning div (.ember-basic-dropdown-content, id="ember-basic-
    // dropdown-content-…") that carries no role at all — role="listbox"
    // only lands on a nested .ember-power-select-options descendant, and
    // that descendant doesn't even exist in the DOM until the dropdown is
    // opened for the first time (before that the wrapper is an empty
    // "…-placeholder" div). Either way, per the ARIA combobox pattern the
    // element the reference points at has no valid popup role.
    ['aria-controls', 'aria-owns'].forEach((attr) => {
      element.querySelectorAll(`[role="combobox"][${attr}]`).forEach((combobox) => {
        const targetId = combobox.getAttribute(attr);
        const popup = document.getElementById(targetId);

        if (!popup || POPUP_ROLES.includes(popup.getAttribute('role'))) {
          return;
        }
        const listbox = popup.querySelector(POPUP_ROLES.map((role) => `[role="${role}"]`).join(', '));
        if (listbox) {
          // The dropdown has been opened at least once, so the real listbox
          // exists inside the wrapper — point the reference straight at it
          // instead of the unroled wrapper around it.
          if (!listbox.id) {
            listbox.id = `${targetId}-listbox`;
          }
          combobox.setAttribute(attr, listbox.id);
        } else {
          // Still closed (nothing rendered inside the wrapper yet, so
          // there's no nested listbox to redirect to) — the wrapper itself
          // is what's exposed as the popup, so it needs the role directly.
          // Harmless once the dropdown does open: the branch above then
          // redirects to the real listbox and this role goes unused.
          popup.setAttribute('role', 'listbox');
        }
      });
    });

    // Remove invalid aria-controls and add missing aria-expanded
    element.querySelectorAll('[aria-controls]').forEach((el) => {
      const controlsId = el.getAttribute('aria-controls');
      const dropdown = document.getElementById(controlsId);

      if (!dropdown) {
        el.removeAttribute('aria-controls');
        if (el.getAttribute('role') === 'combobox') {
          el.setAttribute('aria-expanded', 'false');
        }
      } else if (el.getAttribute('role') === 'combobox' && !el.hasAttribute('aria-expanded')) {
        el.setAttribute('aria-expanded', dropdown.offsetParent !== null ? 'true' : 'false');
      }
    });

    // Add missing aria-label to listboxes
    element.querySelectorAll('[role="listbox"]').forEach((listbox) => {
      if (!listbox.hasAttribute('aria-label')) {
        listbox.setAttribute('aria-label', 'Available Options');
      }
      // Make listbox keyboard accessible
      if (!listbox.hasAttribute('tabindex')) {
        listbox.setAttribute('tabindex', '0');
      }
    });
  }

  setTimeout(fixAria, 100);
  new MutationObserver(fixAria).observe(element, { childList: true, subtree: true });
});
