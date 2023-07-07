import Controller from '@ember/controller';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

export default class ServicesLinkController extends Controller {
  @service router;

  @tracked encodedMagic;
  @tracked isModalOpen = true;
  @tracked successfulLink = false;
  @tracked failedLink = false;

  get decodedMagic() {
    if (this.encodedMagic) {
      return JSON.parse(atob(this.encodedMagic));
    }

    return {};
  }

  get orgName() {
    return this.decodedMagic?.organizationName;
  }

  get projectName() {
    return this.decodedMagic?.projectName;
  }

  get clusterId() {
    return this.decodedMagic?.clusterId;
  }

  @action
  closeLinkModal() {
    this.isModalOpen = false;
    this.router.transitionTo('dc.services.index');
  }

  @action
  async linkAndRestart() {
    const myInit = {
      method: 'PUT',
      headers: {
        Accept: 'application/json',
      },
      body: JSON.stringify({
        resource_id: this.decodedMagic?.resourceLinkUrl,
        client_id: this.decodedMagic?.clientId,
        client_secret: this.decodedMagic?.clientSecret,
      }),
    };

    try {
      const myRequest = new Request('/v1/cloud/link');
      const res = await fetch(myRequest, myInit);
      console.log(res);
      if (res.status === 200 || res.status === 201) {
        this.successfulLink = true;
      } else {
        this.failedLink = true;
      }
    } catch (e) {
      this.failedLink = true;
      console.error(e);
    }

    this.isModalOpen = false;
  }
}
