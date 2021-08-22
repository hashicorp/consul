import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { get, action } from '@ember/object';

// TODO: We should potentially move all these nspace related things
// up a level to application.js

export default class DcRoute extends Route {
  @service('repository/dc') repo;
  @service('repository/permission') permissionsRepo;
  @service('repository/nspace/disabled') nspacesRepo;
  @service('settings') settingsRepo;

  async model(params) {
    let [token, nspace, dc] = await Promise.all([
      this.settingsRepo.findBySlug('token'),
      this.nspacesRepo.getActive(this.optionalParams().nspace),
      this.repo.findBySlug(params.dc),
    ]);

    // When disabled nspaces is [], so nspace is undefined
    const permissions = await this.permissionsRepo.findAll({
      dc: params.dc,
      ns: get(nspace || {}, 'Name'),
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
