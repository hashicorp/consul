import { modifier } from 'ember-modifier';

export default modifier(function fixSuperSelectAria(element) {
  function fixAria() {
    // Fix role="alert" → role="option" on select options
    element.querySelectorAll('[role="alert"][aria-selected]').forEach((el) => {
      el.setAttribute('role', 'option');
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
