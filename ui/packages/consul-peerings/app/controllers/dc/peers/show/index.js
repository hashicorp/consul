/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from "@ember/controller";
import { inject as service } from "@ember/service";
import { action } from "@ember/object";

export default class DcPeersEditIndexController extends Controller {
  @service router;

  @action transitionToImported() {
    this.router.replaceWith("dc.peers.show.imported");
  }
}
