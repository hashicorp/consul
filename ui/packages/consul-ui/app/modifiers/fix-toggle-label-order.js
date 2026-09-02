/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';

export default modifier(function fixToggleLabelOrder(element) {
  function fixOrder() {
    // HDS's <Hds::Form::Field> (which backs Toggle::Field, Checkbox::Field,
    // Radio::Field, …) always renders its <label> before the control's
    // wrapper in the DOM — <label>…</label><div class="hds-form-field__
    // control"><input …></div> — regardless of @layout. For the "flag"
    // layout (checkboxes, radios, toggles) that puts the switch/box to the
    // left and its label to the right, this DOM order fails the checkbox/
    // radio-specific convention that the label should come *after* the
    // control, not before.
    //
    // The "flag" layout's visual position comes entirely from CSS Grid
    // named areas (`grid-template-areas: "control label" …`), independent
    // of source order, and the label is associated to its control purely
    // via for/id (not by wrapping it) — so physically moving the <label>
    // node after the control in the DOM is a no-op visually and for the
    // label/control relationship, while fixing the DOM order the checker
    // (and screen readers reading linearly) actually see.
    element.querySelectorAll('.hds-form-field--layout-flag').forEach((field) => {
      const input = field.querySelector('input[type="checkbox"], input[type="radio"]');
      const label = field.querySelector(':scope > .hds-form-field__label');
      const control = field.querySelector(':scope > .hds-form-field__control');

      if (!input || !label || !control) {
        return;
      }
      if (control.nextElementSibling !== label) {
        control.after(label);
      }
    });
  }

  setTimeout(fixOrder, 100);
  const observer = new MutationObserver(fixOrder);
  observer.observe(element, { childList: true, subtree: true });

  return () => observer.disconnect();
});
