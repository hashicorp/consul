/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Helper from '@ember/component/helper';
import { inject as service } from '@ember/service';
import { getOwner } from '@ember/application';

export default class RefreshRouteHelper extends Helper {
  @service('router') router;

  compute(params, hash) {
    return () => {
      const container = getOwner(this);
      const routeName = this.router.currentRoute.name;
      return container.lookup(`route:${routeName}`).refresh();
    };
  }
}
