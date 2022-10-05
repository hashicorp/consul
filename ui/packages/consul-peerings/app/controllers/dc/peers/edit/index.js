import Controller from "@ember/controller";
import { inject as service } from "@ember/service";
import { action } from "@ember/object";

export default class DcPeersEditIndexController extends Controller {
  @service router;

  @action transitionToStartSubRouteByType(peerModel) {
    if (peerModel.isDialer) {
      this.router.replaceWith("dc.peers.edit.exported");
    } else {
      this.router.replaceWith("dc.peers.edit.imported");
    }
  }
}
