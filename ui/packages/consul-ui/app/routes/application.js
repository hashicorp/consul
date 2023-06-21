/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Route from 'consul-ui/routing/route';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';

import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default class ApplicationRoute extends Route.extend(WithBlockingActions) {
  @service('client/http') client;
  @service('env') env;
  @service() hcp;

  data;

  async model() {
    if (this.env.var('CONSUL_ACLS_ENABLED')) {
      await this.hcp.updateTokenIfNecessary(this.env.var('CONSUL_HTTP_TOKEN'));
    }
    return {};
  }

  @action
  onClientChanged(e) {
    let data = e.data;
    if (data === '') {
      data = { blocking: true };
    }
    // this.data is always undefined first time round and its the 'first read'
    // of the value so we don't need to abort anything
    if (typeof this.data === 'undefined') {
      this.data = Object.assign({}, data);
      return;
    }
    if (this.data.blocking === true && data.blocking === false) {
      this.client.abort();
    }
    this.data = Object.assign({}, data);
  }

  @action
  error(e, transition) {
    // TODO: Normalize all this better
    let error = {
      status: e.code || e.statusCode || '',
      message: e.message || e.detail || 'Error',
    };
    if (e.errors && e.errors[0]) {
      error = e.errors[0];
      error.message = error.message || error.title || error.detail || 'Error';
    }
    if (error.status === '') {
      error.message = 'Error';
    }
    this.controllerFor('application').setProperties({ error: error });
    return true;
  }
}
