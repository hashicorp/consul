/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { assert } from '@ember/debug';
import { inject as service } from '@ember/service';

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
