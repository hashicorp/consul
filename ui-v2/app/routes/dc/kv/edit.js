import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';
import WithKvActions from 'consul-ui/mixins/kv/with-actions';

import ascend from 'consul-ui/utils/ascend';

export default Route.extend(WithKvActions, {
  repo: service('kv'),
  sessionRepo: service('session'),
  beforeModel: function(transition) {
    const url = get(transition, 'intent.url');
    const search = '/edit/';
    if (url && url.endsWith(search)) {
      return this.replaceWith('dc.kv.folder', this.paramsFor(this.routeName).key + search);
    }
  },
  model: function(params) {
    const key = params.key;
    const dc = this.modelFor('dc').dc.Name;
    const repo = get(this, 'repo');
    return hash({
      isLoading: false,
      parent: repo.findBySlug(ascend(key, 1) || '/', dc),
      item: repo.findBySlug(key, dc),
    }).then(model => {
      // TODO: Consider loading this after initial page load
      const session = get(model.item, 'Session');
      if (session) {
        return hash({
          ...model,
          ...{
            session: get(this, 'sessionRepo').findByKey(session, dc),
          },
        });
      }
      return model;
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
