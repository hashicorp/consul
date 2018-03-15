import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import rootKey from 'consul-ui/utils/rootKey';
import transitionToNearestParent from 'consul-ui/utils/transitionToNearestParent';
import ascend from 'consul-ui/utils/ascend';

const prefix = function(key, prefix) {
  // the user provided 'key' form the input field
  // doesn't contain the entire path
  // if its not root, prefix the parentKey (i.e. the folder the user is in)
  if (prefix !== '/') {
    key.set('Key', prefix + key.get('Key'));
  }
  return key;
};
export default Route.extend({
  repo: service('kv'),
  model: function(params) {
    const repo = this.get('repo');
    const key = rootKey(params.key, this.rootKey) || this.rootKey;
    return hash({
      keys: repo.findAllBySlug(key, this.modelFor('dc').dc),
      newKey: repo.create(),
      parentKey: ascend(key, 1) || '/',
      grandParentKey: ascend(key, 2) || '/',
      isLoading: false,
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    create: function(key, parentKey, grandParentKey) {
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
    deleteFolder: function(parentKey, grandParent) {
      this.get('feedback').execute(
        () => {
          // TODO: Possibly change to ember-data entity rather than a pojo
          return this.get('repo')
            .remove({
              Key: parentKey,
            })
            .then(response => {
              return transitionToNearestParent.bind(this)(
                this.modelFor('dc').dc,
                grandParent,
                this.get('rootKey')
              );
            });
        },
        `Deleted ${parentKey}`,
        `There was an error deleting ${parentKey}`
      );
    },
  },
});
