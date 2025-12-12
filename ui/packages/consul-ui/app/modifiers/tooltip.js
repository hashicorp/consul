/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';
import tippy, { followCursor } from 'tippy.js';

/**
 * Tooltip modifier using Tippy.js
 * https://atomiks.github.io/tippyjs
 *
 * {{tooltip 'Text' options=(hash )}}
 */
export default modifier(($element, [content], hash = {}) => {
  if (typeof content === 'string' && content.trim() === '') {
    return;
  }
  const userOptions = hash.options || {};
  const options = { ...userOptions };

  let $anchor = $element;

  // make it easy to specify the modified element as the actual tooltip
  if (typeof options.triggerTarget === 'string') {
    const $el = $anchor;
    switch (options.triggerTarget) {
      case 'parentNode':
        $anchor = $anchor.parentNode;
        break;
      default:
        $anchor = $anchor.querySelectorAll(options.triggerTarget);
    }
    content = $anchor.cloneNode(true);
    $el.remove();
    delete options.triggerTarget;
  }
  // {{tooltip}} will just use the HTML content
  if (typeof content === 'undefined') {
    content = $anchor.innerHTML;
    $anchor.innerHTML = '';
  }
  let interval;
  if (options.trigger === 'manual') {
    // if we are manually triggering, a out delay means only show for the
    // amount of time specified by the delay
    const delay = options.delay || [];
    if (typeof delay[1] !== 'undefined') {
      options.onShown = (tooltip) => {
        clearInterval(interval);
        interval = setTimeout(() => {
          tooltip.hide();
        }, delay[1]);
      };
    }
  }
  let $trigger = $anchor;
  let needsTabIndex = false;
  if (!$trigger.hasAttribute('tabindex')) {
    needsTabIndex = true;
    $trigger.setAttribute('tabindex', '0');
  }
  const tooltip = tippy($anchor, {
    theme: 'tooltip',
    triggerTarget: $trigger,
    content: ($anchor) => content,
    // showOnCreate: true,
    // hideOnClick: false,
    plugins: [typeof options.followCursor !== 'undefined' ? followCursor : undefined].filter(
      (item) => Boolean(item)
    ),
    ...options,
  });

  return () => {
    if (needsTabIndex) {
      $trigger.removeAttribute('tabindex');
    }
    clearInterval(interval);
    tooltip.destroy();
  };
});
