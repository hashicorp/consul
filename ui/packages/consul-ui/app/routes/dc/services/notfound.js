/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

export default class NotfoundRoute extends Route {
  @service router;

  redirect(model, transition) {
    this.router.replaceWith('dc.services.instance', model.name, model.node, model.id);
  }
}
