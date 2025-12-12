/**
 * Copyright IBM Corp. 2014, 2025
 * SPDX-License-Identifier: BUSL-1.1
 */
import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class DcIntentionsCreateController extends Controller {
  @service router;

  @action
  goToIndex(dc) {
    this.router.transitionTo('dc.intentions.index', dc);
  }
}
