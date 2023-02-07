import { modifier } from 'ember-modifier';

export default modifier(function enabled($element, [bool = true], hash) {
  if (['input', 'textarea', 'select', 'button'].includes($element.nodeName.toLowerCase())) {
    if (bool) {
      $element.setAttribute('disabled', bool);
      $element.setAttribute('aria-disabled', bool);
    } else {
      $element.dataset.disabled = false;
      $element.removeAttribute('disabled');
      $element.removeAttribute('aria-disabled');
    }
    return;
  }
  for (const $el of $element.querySelectorAll('input,textarea,button')) {
    if (bool && $el.dataset.disabled !== 'false') {
      $element.setAttribute('disabled', bool);
      $element.setAttribute('aria-disabled', bool);
    } else {
      $element.removeAttribute('disabled');
      $element.removeAttribute('aria-disabled');
    }
  }
});
