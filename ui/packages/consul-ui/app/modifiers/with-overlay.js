/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { modifier } from 'ember-modifier';
import tippy, { followCursor } from 'tippy.js';

/**
 * Overlay modifier using Tippy.js
 * https://atomiks.github.io/tippyjs
 *
 * {{tooltip 'Text' options=(hash )}}
 */
export default modifier(($element, [content], hash = {}) => {
  const options = hash.options || {};

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
    hash.options.triggerTarget = undefined;
  }
  // {{tooltip}} will just use the HTML content
  if (typeof content === 'undefined') {
    content = $anchor.innerHTML;
    $anchor.innerHTML = '';
  }
  if (hash.returns) {
    options.trigger = 'manual';
  }
  let interval;
  if (options.trigger === 'manual') {
    // if we are manually triggering, a out delay means only show for the
    // amount of time specified by the delay
    const delay = options.delay || [];
    if (typeof delay[1] !== 'undefined') {
      options.onShown = (popover) => {
        clearInterval(interval);
        interval = setTimeout(() => {
          popover.hide();
        }, delay[1]);
      };
    }
  }
  let $trigger = $anchor;
  const popover = tippy($anchor, {
    triggerTarget: $trigger,
    content: ($anchor) => content,
    // showOnCreate: true,
    // hideOnClick: false,
    interactive: true,
    plugins: [typeof options.followCursor !== 'undefined' ? followCursor : undefined].filter(
      (item) => Boolean(item)
    ),
    ...options,
  });
  if (hash.returns) {
    hash.returns(popover);
  }
  return () => {
    clearInterval(interval);
    popover.destroy();
  };
});
