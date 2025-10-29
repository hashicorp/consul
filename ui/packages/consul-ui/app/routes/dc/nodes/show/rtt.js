/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */
import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

export default class DcNodesShowRttRoute extends Route {
  redirect(model) {
    const distances = model?.tomography?.distances;
    if (Array.isArray(distances) && distances.length == 0) {
      this.replaceWith('dc.nodes.show');
    }
  }
}