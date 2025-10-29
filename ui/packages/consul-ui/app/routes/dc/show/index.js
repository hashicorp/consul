/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */
import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

export default class DcShowIndexRoute extends Route {
  // Use the abilities service (ember-can) backing the {{can}} helper
  @service abilities;

  afterModel() {
    const canAccess = this.abilities.can('access overview');
    this.replaceWith(canAccess ? 'dc.show.serverstatus' : 'dc.services.index');
  }
}
