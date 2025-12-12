/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from './index';
import { inject as service } from '@ember/service';

export default class FolderRoute extends Route {
  @service router;

  beforeModel(transition) {
    super.beforeModel(...arguments);
    const params = this.paramsFor('dc.kv.folder');
    if (params.key === '/' || params.key == null) {
      return this.router.transitionTo('dc.kv.index');
    }
  }
}
