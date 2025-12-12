/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class DcNodesShowIndexController extends Controller {
  @service router;

  @action
  goServices() {
    this.router.replaceWith('dc.nodes.show.services');
  }

  @action
  goHealthChecks() {
    this.router.replaceWith('dc.nodes.show.healthchecks');
  }
}
