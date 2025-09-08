import { modifier } from 'ember-modifier';

export default modifier(function fixPowerSelectWithCreateAria() {
  function fixAria() {
    // Fix invalid roles and attributes
    document
      .querySelectorAll('[role="alert"][aria-selected], [role="combobox"], [role="listbox"]')
      .forEach((el) => {
        const role = el.getAttribute('role');

        if (role === 'alert') {
          el.setAttribute('role', 'option');
        }

        if (role === 'combobox') {
          const dropdown = document.getElementById(el.getAttribute('aria-controls'));
          if (!dropdown) {
            el.removeAttribute('aria-controls');
          } else {
            el.setAttribute('aria-expanded', dropdown.offsetParent ? 'true' : 'false');
          }
        }

        if (role === 'listbox') {
          if (!el.hasAttribute('aria-label') && !el.hasAttribute('aria-labelledby')) {
            el.setAttribute('aria-label', 'Available options');
          }
          el.removeAttribute('aria-controls');
        }
      });

    // Following is a temporary fix. We should fix this colour contrast issue globally.

    // Fix color contrast for all PowerSelect options
    document.querySelectorAll('.ember-power-select-option').forEach((option) => {
      option.classList.add('consul-powerselect-fixed');
    });
  }

  setTimeout(fixAria, 50);
  new MutationObserver(fixAria).observe(document.body, { childList: true, subtree: true });
});
