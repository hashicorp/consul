import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class ServicesLinkController extends Controller {
  @service router;

  @tracked encodedMagic;

  get decodedMagic() {
    return JSON.parse(atob(this.encodedMagic));
  }

  get orgId() {
    return this.decodedMagic?.organizationId;
  }

  get projectId() {
    return this.decodedMagic?.projectId;
  }

  get clusterId() {
    return this.decodedMagic?.clusterId;
  }

  @action
  closeLinkModal() {
    console.log('here');
    this.router.transitionTo('dc.services.index');
  }
}
