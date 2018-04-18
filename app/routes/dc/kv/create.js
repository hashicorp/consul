import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import WithFeedback from 'consul-ui/mixins/with-feedback';
import { get, set } from '@ember/object';

export default Route.extend(WithFeedback, {
  templateName: 'dc/kv/edit',
  repo: service('kv'),
  model: function(params) {
    const key = params.key;
    const repo = get(this, 'repo');
    const dc = this.modelFor('dc').dc.Name;
    const item = repo.create();
    set(item, 'Datacenter', dc);
    return hash({
      create: true,
      parent: repo.findBySlug(key, dc),
      item: item,
      isLoading: false,
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    create: function(item, parent) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .persist(item)
            .then(item => {
              return this.transitionTo('dc.kv.folder', get(parent, 'Key'));
            });
        },
        `Your key has been added.`,
        `There was an error adding your key.`
      );
    },
    // deleteFolder: function(parentKey, grandParent) {
    //   this.get('feedback').execute(
    //     () => {
    //       const dc = this.modelFor('dc').dc.Name;
    //       return this.get('repo')
    //         .remove({
    //           Key: parentKey,
    //           Datacenter: dc,
    //         })
    //         .then(response => {
    //           return transitionToNearestParent.bind(this)(dc, grandParent, this.get('rootKey'));
    //         });
    //     },
    //     `Deleted ${parentKey}`,
    //     `There was an error deleting ${parentKey}`
    //   );
    // },
  },
});
