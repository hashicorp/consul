/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { schedule } from '@ember/runloop';

export default class DcNodesShowIndexController extends Controller {
  @service router;

  @action
  goServices() {
    // 2. Wrap the transition in schedule
    schedule('actions', () => {
      this.router.replaceWith('dc.nodes.show.services');
    });
  }

  @action
  goHealthChecks() {
    // 3. Wrap this transition as well
    schedule('actions', () => {
      this.router.replaceWith('dc.nodes.show.healthchecks');
    });
  }
}
