/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';

export default class CreateController extends Controller {
  @service router;

  // The Establish peering tab persists the item itself (unlike Generate
  // token, which only ever previews one), so its destination depends on
  // the item the write actually succeeded with.
  @action
  onEstablish(item) {
    this.router.transitionTo('dc.peers.show', item.Name);
  }
}
