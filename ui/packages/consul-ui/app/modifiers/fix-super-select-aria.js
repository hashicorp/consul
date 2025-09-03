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
      } else if (el.getAttribute('role') === 'combobox' && !el.hasAttribute('aria-expanded')) {
        el.setAttribute('aria-expanded', dropdown.offsetParent !== null ? 'true' : 'false');
      }
    });

    // Add accessible names to listboxes
    element
      .querySelectorAll('[role="listbox"]:not([aria-label]):not([aria-labelledby])')
      .forEach((listbox) => {
        listbox.setAttribute('aria-label', 'Available options');
      });
  }

  setTimeout(fixAria, 100);
  new MutationObserver(fixAria).observe(element, { childList: true, subtree: true });
});
