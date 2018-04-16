import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import WithFeedback from 'consul-ui/mixins/with-feedback';
import { get } from '@ember/object';

export default Route.extend(WithFeedback, {
  templateName: 'dc/kv/edit',
  repo: service('kv'),
  model: function(params) {
    const key = params.key;
    const repo = this.get('repo');
    const dc = this.modelFor('dc').dc;
    const item = repo.create();
    item.set('Datacenter', dc);
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
      this.get('feedback').execute(
        () => {
          return get(this, 'repo')
            .persist(item)
            .then(item => {
              this.transitionTo('dc.kv.folder', get(parent, 'Key'));
            });
        },
        `Created ${get(item, 'Key')}`,
        `There was an error creating ${get(item, 'Key')}`
      );
    },
    // deleteFolder: function(parentKey, grandParent) {
    //   this.get('feedback').execute(
    //     () => {
    //       const dc = this.modelFor('dc').dc;
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
