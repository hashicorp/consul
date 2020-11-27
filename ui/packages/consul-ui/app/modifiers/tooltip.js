import { modifier } from 'ember-modifier';
import tippy, { followCursor } from 'tippy.js';

/**
 * Tooltip modifier using Tippy.js
 * https://atomiks.github.io/tippyjs
 *
 * {{tooltip 'Text' options=(hash )}}
 */
export default modifier(($element, [content], hash = {}) => {
  const options = hash.options || {};
  let $target = $element;

  // make it easy to specify the modified element as the actual tooltip
  if (typeof options.triggerTarget === 'string') {
    const $el = $target;
    switch (options.triggerTarget) {
      case 'parentNode':
        $target = $target.parentNode;
        break;
      default:
        $target = $target.querySelectorAll(options.triggerTarget);
    }
    content = $target.cloneNode(true);
    $el.remove();
    hash.options.triggerTarget = undefined;
  }
  // {{tooltip}} will just use the HTML content
  if (typeof content === 'undefined') {
    content = $target.innerHTML;
    $target.innerHTML = '';
  }
  let interval;
  if (options.trigger === 'manual') {
    // if we are manually triggering, a out delay means only show for the
    // amount of time specified by the delay
    const delay = options.delay || [];
    if (typeof delay[1] !== 'undefined') {
      hash.options.onShown = tooltip => {
        setTimeout(() => {
          tooltip.hide();
        }, delay[1]);
      };
    }
  }
  const tooltip = tippy($target, {
    theme: 'tooltip',
    triggerTarget: $target,
    content: $reference => content,
    // showOnCreate: true,
    // hideOnClick: false,
    plugins: [typeof options.followCursor !== 'undefined' ? followCursor : undefined].filter(item =>
      Boolean(item)
    ),
    ...hash.options,
  });

  return () => {
    clearInterval(interval);
    tooltip.destroy();
  };
});
