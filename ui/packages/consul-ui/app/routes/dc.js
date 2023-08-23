/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';

// TODO: We should potentially move all these nspace related things
// up a level to application.js

export default class DcRoute extends Route {
  @service('repository/permission') permissionsRepo;

  async model(params) {
    // When disabled nspaces is [], so nspace is undefined
    const permissions = await this.permissionsRepo.findAll({
      dc: params.dc,
      ns: this.optionalParams().nspace,
      partition: this.optionalParams().partition,
    });
    // the model here is actually required for the entire application
    // but we need to wait until we are in this route so we know what the dc
    // and or nspace is if the below changes please revisit the comments
    // in routes/application:model
    // We do this here instead of in setupController to prevent timing issues
    // in lower routes
    this.controllerFor('application').setProperties({
      permissions,
    });
    return {
      permissions,
    };
  }
}
