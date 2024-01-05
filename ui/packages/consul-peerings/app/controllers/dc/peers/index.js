/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Controller from "@ember/controller";
import { inject as service } from "@ember/service";

export default class DcPeersIndexController extends Controller {
  @service router;

  redirectToPeerShow = (modalCloseFn, peerModel) => {
    modalCloseFn?.();

    this.router.transitionTo("dc.peers.show", peerModel.Name);
  };
}
