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
      // Make the listbox keyboard accessible itself *unless* a real,
      // searchable <input role="combobox"> already owns it (via
      // aria-controls/aria-owns) — that input drives navigation into this
      // listbox through aria-activedescendant without the listbox ever
      // taking real DOM focus, so giving it its own tabindex on top of
      // that makes the combobox's popup independently tabbable, which
      // fails "only the input may hold focus while the popup is open".
      const hasOwningInput =
        listbox.id &&
        document.querySelector(
          `input[role="combobox"][aria-controls="${listbox.id}"], input[role="combobox"][aria-owns="${listbox.id}"]`
        );
      if (hasOwningInput) {
        listbox.removeAttribute('tabindex');
      } else if (!listbox.hasAttribute('tabindex')) {
        listbox.setAttribute('tabindex', '0');
      }
    });

    // When @searchEnabled renders the dropdown open, ember-power-select puts
    // a *second*, independently-focusable role="combobox" element into the
    // page: a real <input> for typing, alongside the outer trigger div that
    // already carries role="combobox" (and its own tabindex) for the closed
    // state. Two simultaneously-focusable combobox surfaces for one widget
    // fails "only the text input may hold focus while the popup is open" —
    // so once the real input exists, demote the trigger to a plain
    // non-interactive wrapper — a role="generic" element can't carry any of
    // these (aria-labelledby/aria-expanded/aria-controls/aria-owns included
    // — ARIA-in-HTML disallows naming/widget-state attributes on generic
    // elements), so all of them get stripped. Only a subset gets restored
    // by hand once the popup closes: aria-expanded and aria-controls/owns
    // are recomputed by the library's own trigger modifier on every
    // open/close (aria-expanded unconditionally, aria-controls/owns
    // whenever they're null) — so leaving them stripped is enough for that
    // modifier's own logic to fill them back in correctly; restoring stale
    // values here first would just leave a dangling reference until then.
    const DEMOTED_ATTRS = [
      'role',
      'aria-autocomplete',
      'aria-haspopup',
      'aria-expanded',
      'aria-controls',
      'aria-owns',
      'aria-labelledby',
      'tabindex',
    ];
    const RESTORED_ATTRS = ['role', 'aria-autocomplete', 'aria-haspopup', 'aria-labelledby', 'tabindex'];
    element
      .querySelectorAll('.ember-basic-dropdown-trigger[data-ebd-id]')
      .forEach((trigger) => {
        // role="combobox" is gone once demoted, so match on that *or* our
        // own marker — otherwise a demoted trigger falls out of this query
        // entirely and never gets restored once its popup closes.
        if (trigger.getAttribute('role') !== 'combobox' && !trigger.hasAttribute('data-a11y-demoted')) {
          return;
        }
        const uniqueId = trigger.getAttribute('data-ebd-id').replace(/-trigger$/, '');
        const content = document.getElementById(`ember-basic-dropdown-content-${uniqueId}`);
        const input = content && content.querySelector('input[role="combobox"]');

        if (input && !trigger.hasAttribute('data-a11y-demoted')) {
          trigger.setAttribute('data-a11y-demoted', 'true');
          DEMOTED_ATTRS.forEach((attr) => {
            const value = trigger.getAttribute(attr);
            if (value !== null && RESTORED_ATTRS.includes(attr)) {
              trigger.setAttribute(`data-a11y-orig-${attr}`, value);
            }
            trigger.removeAttribute(attr);
          });
          trigger.setAttribute('tabindex', '-1');
        } else if (!input && trigger.hasAttribute('data-a11y-demoted')) {
          trigger.removeAttribute('data-a11y-demoted');
          RESTORED_ATTRS.forEach((attr) => {
            const orig = trigger.getAttribute(`data-a11y-orig-${attr}`);
            if (orig !== null) {
              trigger.setAttribute(attr, orig);
            }
            trigger.removeAttribute(`data-a11y-orig-${attr}`);
          });
        }
      });

    // role="combobox" isn't a valid explicit ARIA role on <input type="search">
    // per the ARIA-in-HTML mapping (only type="text"/no type allows it), but
    // ember-power-select's search input hardcodes type="search". Switching it
    // to type="text" is behaviorally identical here (nothing relies on the
    // native search-input affordances) and makes the role valid.
    element.querySelectorAll('input[role="combobox"][type="search"]').forEach((input) => {
      input.setAttribute('type', 'text');
    });

    // ember-power-select mirrors aria-selected from the *chosen* option only,
    // not the *highlighted* one that aria-activedescendant currently points
    // at (that's tracked separately, non-standardly, via aria-current) — so
    // before anything is chosen, the option referenced by
    // aria-activedescendant often has no aria-selected="true" at all. Keep
    // exactly the referenced option marked selected.
    element.querySelectorAll('[aria-activedescendant]').forEach((combo) => {
      const activeId = combo.getAttribute('aria-activedescendant');
      const activeOption = activeId && document.getElementById(activeId);
      if (!activeOption || activeOption.getAttribute('role') !== 'option') {
        return;
      }
      const listbox = activeOption.closest('[role="listbox"]') || activeOption.parentElement;
      listbox?.querySelectorAll('[role="option"][data-a11y-forced-selected]').forEach((option) => {
        if (option !== activeOption) {
          option.setAttribute('aria-selected', 'false');
          option.removeAttribute('data-a11y-forced-selected');
        }
      });
      if (activeOption.getAttribute('aria-selected') !== 'true') {
        activeOption.setAttribute('aria-selected', 'true');
        activeOption.setAttribute('data-a11y-forced-selected', 'true');
      }
    });
  }

  setTimeout(fixAria, 100);
  new MutationObserver(fixAria).observe(element, {
    childList: true,
    subtree: true,
    attributes: true,
    // Narrow filter: these are the attributes ember-power-select updates as
    // the user types/navigates (not ones only *we* write), so watching them
    // re-runs fixAria on that navigation without the observer re-triggering
    // itself in a loop.
    attributeFilter: ['aria-activedescendant', 'aria-expanded'],
  });
});
