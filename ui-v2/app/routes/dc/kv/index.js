import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';
import WithKvActions from 'consul-ui/mixins/kv/with-actions';

export default Route.extend(WithKvActions, {
  repo: service('kv'),
  model: function(params) {
    const key = params.key || '/';
    const dc = this.modelFor('dc').dc.Name;
    const repo = get(this, 'repo');
    return hash({
      isLoading: false,
      parent: repo.findBySlug(key, dc),
    })
      .then(function(model) {
        return hash({
          ...model,
          ...{
            items: repo.findAllBySlug(get(model.parent, 'Key'), dc),
          },
        });
      })
      .catch(e => {
        if (e.errors && e.errors[0] && e.errors[0].status == '404') {
          this.transitionTo('dc.kv.index');
          return;
        }
        throw e;
      });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
