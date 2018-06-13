import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';
import WithKvActions from 'consul-ui/mixins/kv/with-actions';

export default Route.extend(WithKvActions, {
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  repo: service('kv'),
  model: function(params) {
    const key = params.key || '/';
    const dc = this.modelFor('dc').dc.Name;
    const repo = get(this, 'repo');
    return hash({
      isLoading: false,
      parent: repo.findBySlug(key, dc),
    }).then(model => {
      return hash({
        ...model,
        ...{
          items: repo.findAllBySlug(get(model.parent, 'Key'), dc).catch(e => {
            return this.transitionTo('dc.kv.index');
          }),
        },
      });
    });
  },
  actions: {
    error: function(e) {
      if (e.errors && e.errors[0] && e.errors[0].status == '404') {
        return this.transitionTo('dc.kv.index');
      }
      throw e;
    },
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
