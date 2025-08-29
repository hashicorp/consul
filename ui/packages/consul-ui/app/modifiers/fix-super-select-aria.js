import { modifier } from 'ember-modifier';

export default modifier(function fixSuperSelectAria(element) {
  // Inject CSS fixes once globally
  if (!document.getElementById('hds-contrast-fix')) {
    const style = document.createElement('style');
    style.id = 'hds-contrast-fix';
    style.textContent = `
      /* Fix contrast issues */
      .type-text em,
      .hds-form-field__helper-text,
      .hds-form-super-select__helper-text,
      .hds-form-field em {
        color: #495057 !important;
      }
      
      /* Fix dropdown text visibility */
      .hds-form-super-select__option,
      .hds-form-super-select__option *,
      .ember-power-select-option,
      .ember-power-select-option *,
      .child-selector [role="option"] {
        color: #212529 !important;
        opacity: 1 !important;
        visibility: visible !important;
      }
      
      /* Fix side nav disabled items */
      .hds-side-nav__list-item.consul-disabled-nav,
      .hds-side-nav__list-item.consul-disabled-nav a,
      .hds-side-nav__list-item.consul-disabled-nav span {
        color: #9ca3af !important;
      }
      
      .hds-side-nav__wrapper .consul-disabled-nav {
        color: #d1d5db !important;
      }
    `;
    document.head.appendChild(style);
  }

  function fixAria() {
    // Fix roles and ARIA attributes
    element.querySelectorAll('[role="alert"][aria-selected]').forEach((el) => {
      if (
        el.classList.contains('hds-form-super-select__option') ||
        el.classList.contains('ember-power-select-option')
      ) {
        el.setAttribute('role', 'option');
      } else {
        el.removeAttribute('aria-selected');
      }
    });

    // Fix combobox missing aria-expanded
    element.querySelectorAll('[role="combobox"]:not([aria-expanded])').forEach((el) => {
      // Check if dropdown is open by looking for related elements
      const controlsId = el.getAttribute('aria-controls');
      const isExpanded = controlsId && document.getElementById(controlsId) ? 'true' : 'false';
      el.setAttribute('aria-expanded', isExpanded);
    });

    // Update aria-expanded when dropdown state changes
    element.querySelectorAll('[role="combobox"][aria-controls]').forEach((el) => {
      const controlsId = el.getAttribute('aria-controls');
      if (controlsId) {
        const dropdown = document.getElementById(controlsId);
        const isVisible = dropdown && dropdown.offsetParent !== null;
        el.setAttribute('aria-expanded', isVisible ? 'true' : 'false');
      }
    });

    // Remove invalid aria-controls
    element.querySelectorAll('[aria-controls]').forEach((el) => {
      if (!document.getElementById(el.getAttribute('aria-controls'))) {
        el.removeAttribute('aria-controls');
        // If we remove aria-controls, also set aria-expanded to false
        if (el.getAttribute('role') === 'combobox') {
          el.setAttribute('aria-expanded', 'false');
        }
      }
    });

    // Add missing accessible names
    element
      .querySelectorAll('[role="listbox"]:not([aria-label]):not([aria-labelledby]):not([title])')
      .forEach((el) => el.setAttribute('aria-label', 'Select options'));

    element
      .querySelectorAll('[role="combobox"]:not([aria-label]):not([aria-labelledby]):not([title])')
      .forEach((el) => el.setAttribute('aria-label', 'Select input'));

    // Fix options visibility and accessibility
    element.querySelectorAll('[role="option"]').forEach((option) => {
      if (!option.getAttribute('aria-label') && !option.textContent?.trim()) {
        option.setAttribute('aria-label', 'Select option');
      }
      option.style.cssText = 'color: #212529 !important; opacity: 1 !important;';
      option.querySelectorAll('*').forEach((nested) => {
        nested.style.cssText = 'color: #212529 !important; opacity: 1 !important;';
      });
    });
  }

  // Run fixes with delays and watch for changes
  [0, 100, 500].forEach((delay) => setTimeout(fixAria, delay));
  const observer = new MutationObserver(() => setTimeout(fixAria, 0));
  observer.observe(element, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ['role', 'aria-selected', 'aria-controls', 'aria-label', 'aria-expanded'],
  });

  return () => observer.disconnect();
});
