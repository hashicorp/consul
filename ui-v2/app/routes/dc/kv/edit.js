import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';
import WithKvActions from 'consul-ui/mixins/kv/with-actions';

import ascend from 'consul-ui/utils/ascend';

export default Route.extend(WithKvActions, {
  repo: service('repository/kv'),
  sessionRepo: service('repository/session'),
  model: function(params) {
    const key = params.key;
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);
    return hash({
      isLoading: false,
      parent: this.repo.findBySlug(ascend(key, 1) || '/', dc, nspace),
      item: this.repo.findBySlug(key, dc, nspace),
      session: null,
    }).then(model => {
      // TODO: Consider loading this after initial page load
      const session = get(model.item, 'Session');
      if (session) {
        return hash({
          ...model,
          ...{
            session: this.sessionRepo.findByKey(session, dc, nspace),
          },
        });
      }
      return model;
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
