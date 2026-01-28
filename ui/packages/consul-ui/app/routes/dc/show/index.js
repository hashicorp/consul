/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */
import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

export default class DcShowIndexRoute extends Route {
  @service abilities;
  @service router;

  afterModel() {
    const canAccess = this.abilities.can('access overview');
    this.router.replaceWith(canAccess ? 'dc.show.serverstatus' : 'dc.services.index');
  }
}
