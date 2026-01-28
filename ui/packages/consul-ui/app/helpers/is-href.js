/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { scheduleOnce } from '@ember/runloop';

export default class IsHrefHelper extends Helper {
  @service('router') router;
  constructor(...args) {
    super(...args);
    this.router.on('routeWillChange', this.routeWillChange);
  }

  compute([targetRouteName, ...rest]) {
    if (this.router.currentRouteName.startsWith('nspace.') && targetRouteName.startsWith('dc.')) {
      targetRouteName = `nspace.${targetRouteName}`;
    }
    if (typeof this.next !== 'undefined' && this.next !== 'loading') {
      return this.next.startsWith(targetRouteName);
    }
    return this.router.isActive(...[targetRouteName, ...rest]);
  }

  @action
  routeWillChange(transition) {
    const nextRoute = transition.to.name.replace('.index', '');
    // Defer mutation + recompute without anonymous inline function
    scheduleOnce('afterRender', this, this._commitNext, nextRoute);
  }

  _commitNext(nextRoute) {
    this.next = nextRoute;
    this.recompute();
  }

  willDestroy() {
    this.router.off('routeWillChange', this.routeWillChange);
    super.willDestroy();
  }
}
