import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';
import { get, set, action } from '@ember/object';

export default class SettingsRoute extends Route {
  @service('client/http')
  client;

  @service('settings')
  repo;

  @service('repository/dc')
  dcRepo;

  @service('repository/nspace/disabled')
  nspacesRepo;

  model(params) {
    const app = this.modelFor('application');
    return hash({
      item: this.repo.findAll(),
      dc: this.dcRepo.getActive(undefined, app.dcs),
      nspace: this.nspacesRepo.getActive(),
    }).then(model => {
      if (typeof get(model.item, 'client.blocking') === 'undefined') {
        set(model, 'item.client', { blocking: true });
      }
      return model;
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }

  @action
  update(slug, item) {
    switch (slug) {
      case 'client':
        if (!get(item, 'client.blocking')) {
          this.client.abort();
        }
        break;
    }
    this.repo.persist({
      [slug]: item,
    });
  }
}
