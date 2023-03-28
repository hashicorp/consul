import Component from '@glimmer/component';
import { tracked } from '@glimmer/tracking';
import { hrefTo } from 'consul-ui/helpers/href-to';

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
 *     href=(href-to "some.route")
 *     selected=(is-href "some.route")
 *   )
 *   (hash
 *     label="Second Tab"
 *     href=(href-to "some.route")
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
 *    const owner = getOwner(this);
 *     return [
 *       new Tab({
 *         label: 'First Tab',
 *         route: 'some.route',
 *         currentRouteName: router.currentRouteName,
 *         owner
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
    const { currentRouteName, route, label, tooltip, owner } = opts;

    this.currentRouteName = currentRouteName;
    this.owner = owner;
    this.route = route;
    this.label = label;
    this.tooltip = tooltip;
  }

  get selected() {
    return this.currentRouteName === this.route;
  }

  get href() {
    return hrefTo(this.owner, [this.route]);
  }
}

function noop() {}
export default class TabNav extends Component {
  get onClick() {
    return this.args.onclick || noop;
  }

  get onTabClicked() {
    return this.args.onTabClicked || noop;
  }
}
