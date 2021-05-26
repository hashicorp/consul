import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { get, action } from '@ember/object';

// TODO: We should potentially move all these nspace related things
// up a level to application.js

const findActiveNspace = function(nspaces, nspace) {
  let found = nspaces.find(function(item) {
    return item.Name === nspace.Name;
  });
  if (typeof found === 'undefined') {
    // if we can't find the nspace that was saved
    // try default
    found = nspaces.find(function(item) {
      return item.Name === 'default';
    });
    // if there is no default just choose the first
    if (typeof found === 'undefined') {
      found = nspaces.firstObject;
    }
  }
  return found;
};
export default class DcRoute extends Route {
  @service('repository/dc') repo;
  @service('repository/permission') permissionsRepo;
  @service('repository/nspace/disabled') nspacesRepo;
  @service('settings') settingsRepo;

  async model(params) {
    const app = this.modelFor('application');

    let [token, nspace, dc] = await Promise.all([
      this.settingsRepo.findBySlug('token'),
      this.nspacesRepo.getActive(this.optionalParams().nspace),
      this.repo.findBySlug(params.dc, app.dcs),
    ]);
    // if there is only 1 namespace then use that
    // otherwise find the namespace object that corresponds
    // to the active one
    nspace =
      app.nspaces.length > 1 ? findActiveNspace(app.nspaces, nspace) : app.nspaces.firstObject;

    // When disabled nspaces is [], so nspace is undefined
    const permissions = await this.permissionsRepo.findAll({
      dc: params.dc,
      nspace: get(nspace || {}, 'Name'),
    });
    // the model here is actually required for the entire application
    // but we need to wait until we are in this route so we know what the dc
    // and or nspace is if the below changes please revisit the comments
    // in routes/application:model
    // We do this here instead of in setupController to prevent timing issues
    // in lower routes
    this.controllerFor('application').setProperties({
      dc,
      nspace,
      token,
      permissions,
    });
    return {
      dc,
      nspace,
      token,
      permissions,
    };
  }

  // TODO: This will eventually be deprecated please see
  // https://deprecations.emberjs.com/v3.x/#toc_deprecate-router-events
  @action
  willTransition(transition) {
    if (
      typeof transition !== 'undefined' &&
      (transition.from.name.endsWith('nspaces.create') ||
        transition.from.name.startsWith('nspace.dc.acls.tokens'))
    ) {
      // Only when we create, reload the nspaces in the main menu to update them
      // as we don't block for those
      // And also when we [Use] a token reload the nspaces that you are able to see,
      // including your permissions for being able to manage namespaces
      // Potentially we should just do this on every single transition
      // but then we would need to check to see if nspaces are enabled
      const controller = this.controllerFor('application');
      Promise.all([
        this.nspacesRepo.findAll(),
        this.permissionsRepo.findAll({
          dc: get(controller, 'dc.Name'),
          nspace: get(controller, 'nspace.Name'),
        }),
      ]).then(([nspaces, permissions]) => {
        if (typeof controller !== 'undefined') {
          controller.setProperties({
            nspaces: nspaces,
            permissions: permissions,
          });
        }
      });
    }
  }
}
