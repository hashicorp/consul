/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

export default class UnavailableRoute extends Route {
  @service('env') env;
  @service() router;

  beforeModel() {
    if (!this.env.var('CONSUL_V2_CATALOG_ENABLED')) {
      this.router.replaceWith('index');
    }
  }
}
