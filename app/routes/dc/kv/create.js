import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import WithFeedback from 'consul-ui/mixins/with-feedback';
import ascend from 'consul-ui/utils/ascend';
import rootKey from 'consul-ui/utils/rootKey';


const prefix = function(key, prefix) {
  // the user provided 'key' form the input field
  // doesn't contain the entire path
  // if its not root, prefix the parentKey (i.e. the folder the user is in)
  if (prefix !== '/') {
    key.set('Key', prefix + key.get('Key'));
  }
  return key;
};
export default Route.extend(WithFeedback, {
  templateName: 'dc/kv/edit',
  repo: service('kv'),
  rootKey: '-',
  model: function(params) {
    const key = rootKey(params.key, this.rootKey) || this.rootKey;
    const item = this.get('repo').create();
    item.set('Datacenter', this.modelFor('dc').dc);
    return hash({
      create: true,
      item: item,
      isLoading: false,
      parentKey: ascend(key, 1) || ''
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    create: function(key, parentKey) {
      this.get('feedback').execute(
        () => {
          return this.get('repo')
            .persist(prefix(key, parentKey))
            .then(key => {
              this.transitionTo(key.get('isFolder') ? 'dc.kv.show' : 'dc.kv.edit', key.get('Key'));
            });
        },
        `Created ${key.get('Key')}`,
        `There was an error creating ${key.get('Key')}`
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
