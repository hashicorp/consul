/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */
import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

export default class DcNodesShowRttRoute extends Route {
  @service router;

  redirect(model) {
    const distances = model?.tomography?.distances;
    if (Array.isArray(distances) && distances.length == 0) {
      this.router.replaceWith('dc.nodes.show');
    }
  }
}
