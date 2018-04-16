import Route from '@ember/routing/route';

import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { assign } from '@ember/polyfills';
import { get } from '@ember/object';
import WithFeedback from 'consul-ui/mixins/with-feedback';

export default Route.extend(WithFeedback, {
  repo: service('kv'),
  model: function(params) {
    const key = params.key || '/';
    const dc = this.modelFor('dc').dc;
    const repo = get(this, 'repo');
    return hash({
      isLoading: false,
      parent: repo.findBySlug(key, dc),
    })
      .then(
        function(model) {
          return hash(
            assign({}, model, {
              // better name, slug vs key?
              items: repo.findAllBySlug(get(model.parent, 'Key'), dc),
            })
          );
        }
        // usually when an entire folder structure and no longer exists
        // a 404 comes back, just redirect to root as the old UI did
      )
      .catch(() => {
        // this still gives me an error!?
        return this.transitionTo('dc.kv.index');
      });
  },
  actions: {
    delete: function(item, parent) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .remove(item)
            .then(() => {
              return this.refresh();
            });
        },
        `Deleted ${get(item, 'Key')}`,
        `There was an error deleting ${get(item, 'Key')}`
      );
    },
    // TODO: This is frontend ??
    cancel: function(item, parent) {
      return this.transitionTo('dc.kv.folder', parent);
    },
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
