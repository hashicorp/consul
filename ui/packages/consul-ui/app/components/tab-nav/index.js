/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { assert } from '@ember/debug';
import { inject as service } from '@ember/service';
import { modifier } from 'ember-modifier';
import tippy from 'tippy.js';

// Mirrors the `hideOnEsc` plugin baked into the HDS `{{hds-tooltip}}`
// modifier so keyboard users can dismiss the tooltip the same way they can
// everywhere else in the app.
const hideOnEsc = {
  name: 'hideOnEsc',
  defaultValue: true,
  fn({ hide }) {
    function onKeyDown(event) {
      if (event.key === 'Escape') {
        hide();
      }
    }
    return {
      onShow() {
        document.addEventListener('keydown', onKeyDown);
      },
      onHide() {
        document.removeEventListener('keydown', onKeyDown);
      },
    };
  },
};

/**
 * A class that encapsulates the data abstraction that we expect the TabNav to
 * be passed as `@items`.
 *
 * You can use this class when you want to create tab-nav from javascript.
 *
 * Instead of doing this in the template layer:
 *
 * ```handlebars
 * <TabNav @items={{array
 *   (hash
 *     label="First Tab"
 *     route="some.route"
 *     selected=(is-href "some.route")
 *   )
 *   (hash
 *     label="Second Tab"
 *     route="some.other-route"
 *     selected=(is-href "some.route")
 *   )
 * }}
 * ```
 *
 * You can do the following in a js-file:
 *
 * ```javascript
 * export default class WootComponent extends Component {
 *   // ...
 *   get tabs() {
 *    const { router } = this;
 *     return [
 *       new Tab({
 *         label: 'First Tab',
 *         route: 'some.route',
 *         currentRouteName: router.currentRouteName
 *        }),
 *       // ...
 *     ];
 *   }
 * }
 * ```
 *
 */
export class Tab {
  @tracked route;
  @tracked label;
  @tracked tooltip;
  @tracked currentRouteName;

  constructor(opts) {
    const { currentRouteName, route, label, tooltip } = opts;

    this.currentRouteName = currentRouteName;
    this.route = route;
    this.label = label;
    this.tooltip = tooltip;
  }

  get selected() {
    return this.currentRouteName === this.route;
  }
}

function noop() {}
export default class TabNav extends Component {
  @service router;

  // Hds::Tabs renders each tab's yielded content inside its own internal
  // `<button role="tab">`; `...attributes`/modifiers applied to `<T.Tab>`
  // land on the wrapping `<li>`, not that button, so `{{hds-tooltip}}`
  // can't be attached to the button directly. Wrapping the label in a
  // `tabindex="-1"` `<span>` to trigger it instead nests an
  // (unreachable-by-keyboard) tooltip trigger inside the already-focusable
  // tab button. Attach a real tippy instance straight to the actual tab
  // button found in the DOM so it participates in the normal focus/hover
  // triggers, matching the "hds" tooltip theme for visual consistency.
  attachTabTooltip = modifier((li, [tooltip]) => {
    if (!tooltip) {
      return;
    }
    const button = li.querySelector('.hds-tabs__tab-button');
    if (!button) {
      return;
    }
    const instance = tippy(button, {
      content: tooltip,
      theme: 'hds',
      interactive: true,
      aria: { expanded: false },
      arrow: `
        <svg class="hds-tooltip-pointer" width="16" height="7" viewBox="0 0 16 7" xmlns="http://www.w3.org/2000/svg">
          <path d="M0 7H16L9.11989 0.444571C8.49776 -0.148191 7.50224 -0.148191 6.88011 0.444572L0 7Z" />
        </svg>`,
      plugins: [hideOnEsc],
    });
    return () => instance.destroy();
  });

  get onClick() {
    return this.args.onclick || noop;
  }

  get onTabClicked() {
    return this.args.onTabClicked || noop;
  }

  itemAt(index) {
    return typeof this.args.items.objectAt === 'function'
      ? this.args.items.objectAt(index)
      : this.args.items[index];
  }

  get selectedTabIndex() {
    for (let index = 0; index < this.args.items.length; index++) {
      if (this.itemAt(index).selected) {
        return index;
      }
    }
    return 0;
  }

  get tabsKey() {
    const keys = [];
    for (let index = 0; index < this.args.items.length; index++) {
      const item = this.itemAt(index);
      keys.push(item.route || item.label);
    }
    return keys.join('|');
  }

  onClickTab = (event) => {
    const tab = event.currentTarget.parentElement;
    const tabs = tab.parentElement.querySelectorAll(':scope > .hds-tabs__tab');
    const index = Array.from(tabs).indexOf(tab);
    const item = this.itemAt(index);

    assert('TabNav could not match the selected HDS tab to an item', index !== -1 && item);
    assert(
      'TabNav items require a route or an interaction callback',
      item.route || this.args.onclick || this.args.onTabClicked
    );

    this.onClick(item.label.toUpperCase(), event);
    this.onTabClicked(item, event);

    if (item.route) {
      this.router.transitionTo(item.route);
    }
  };
}
