import { modifier } from 'ember-modifier';

export default modifier(function enabled($element, [bool = true], hash) {
  if (['input', 'textarea', 'select', 'button'].includes($element.nodeName.toLowerCase())) {
    if (bool) {
      $element.disabled = bool;
    } else {
      $element.dataset.disabled = false;
    }
    return;
  }
  for (const $el of $element.querySelectorAll('input,textarea')) {
    if ($el.dataset.disabled !== 'false') {
      $el.disabled = bool;
    }
  }
});
