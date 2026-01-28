/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from "@ember/controller";
import { inject as service } from "@ember/service";
import { action } from "@ember/object";
import { schedule } from "@ember/runloop";

export default class DcPeersEditIndexController extends Controller {
  @service router;

  @action transitionToImported() {
    schedule('afterRender', this, () => {
      this.router.replaceWith("dc.peers.show.imported");
    });
  }
}
