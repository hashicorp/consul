import { modifier } from 'ember-modifier';

export default modifier(function fixCodeBlockAria(element) {
  function fixAria() {
    // Fix HDS CodeBlock ARIA issue - add role to pre elements with aria-labelledby
    // This is fixed in newer versions of HDS and can be removed when we upgrade HDS
    element.querySelectorAll('pre[aria-labelledby]:not([role])').forEach((pre) => {
      pre.setAttribute('role', 'region');
    });
  }

  setTimeout(fixAria, 100);
  new MutationObserver(fixAria).observe(element, { childList: true, subtree: true });
});
