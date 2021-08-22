import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';
import { get, action } from '@ember/object';

export default class SettingsRoute extends Route {
  @service('client/http') client;

  @service('settings') repo;
  @service('repository/dc') dcRepo;
  @service('repository/permission') permissionsRepo;
  @service('repository/nspace/disabled') nspacesRepo;

  async model(params) {
    // reach up and grab things from the application route/controller
    const app = this.controllerFor('application');

    // figure out if we have anything missing for menus etc and get them if
    // so, otherwise just use what they already are
    const [item, dc] = await Promise.all([
      this.repo.findAll(),
      typeof app.dc === 'undefined' ? this.dcRepo.getActive() : app.dc,
    ]);
    const nspace =
      typeof app.nspace === 'undefined'
        ? await this.nspacesRepo.getActive(item.nspace)
        : app.nspace;
    const permissions =
      typeof app.permissions === 'undefined'
        ? await this.permissionsRepo.findAll({
            dc: dc.Name,
            ns: nspace.Name,
          })
        : app.permissions;

    // reset the things higher up in the application if they were already set
    // this won't do anything
    this.controllerFor('application').setProperties({
      dc: dc,
      nspace: nspace,
      token: item.token,
      permissions: permissions,
    });

    if (typeof get(item, 'client.blocking') === 'undefined') {
      item.client = { blocking: true };
    }
    return { item };
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
