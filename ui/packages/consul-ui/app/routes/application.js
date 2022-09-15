import Route from 'consul-ui/routing/route';
import { action } from '@ember/object';
import { inject as service } from '@ember/service';
import { runInDebug } from '@ember/debug';

import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default class ApplicationRoute extends Route.extend(WithBlockingActions) {
  @service('client/http') client;
  @service('env') env;
  @service('repository/token') tokenRepo;
  @service('settings') settings;

  data;

  async model() {
    if (this.env.var('CONSUL_ACLS_ENABLED')) {
      const secret = this.env.var('CONSUL_HTTP_TOKEN');
      const existing = await this.settings.findBySlug('token');
      if (!existing.AccessorID && secret) {
        try {
          const token = await this.tokenRepo.self({
            secret: secret,
            dc: this.env.var('CONSUL_DATACENTER_LOCAL'),
          });
          await this.settings.persist({
            token: {
              AccessorID: token.AccessorID,
              SecretID: token.SecretID,
              Namespace: token.Namespace,
              Partition: token.Partition,
            },
          });
        } catch (e) {
          runInDebug((_) => console.error(e));
        }
      }
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
