/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

// Thin wrapper around the `href-to` helper that accepts a pre-built models
// array rather than variadic positional arguments.
//
// Usage in templates:
//   @href={{breadcrumb-href item.route item.models}}
//
// This avoids needing to spread `item.models` (which HBS does not support)
// when calling `(href-to route model0 model1 ...)`.

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { getOwner } from '@ember/application';
import { scheduleOnce } from '@ember/runloop';
import { hrefTo } from 'consul-ui/helpers/href-to';

export default class BreadcrumbHrefHelper extends Helper {
  @service('router') router;

  constructor(...args) {
    super(...args);
    this.router.on('routeWillChange', this.routeWillChange);
  }

  compute([route, models = []]) {
    if (!route) return undefined;
    // `hrefTo` expects params as [routeName, ...models]
    return hrefTo(getOwner(this), [route, ...models]);
  }

  @action
  routeWillChange() {
    scheduleOnce('afterRender', this, 'recompute');
  }

  willDestroy() {
    this.router.off('routeWillChange', this.routeWillChange);
    super.willDestroy();
  }
}
