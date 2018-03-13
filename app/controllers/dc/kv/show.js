import Controller from '@ember/controller';

import confirm from 'consul-ui/utils/confirm';

export default Controller.extend({
  actions: {
    requestDeleteFolder: function(parentKey, grandParent) {
      confirm('Are you sure you want to delete this folder?')
        .then(() => {
          return this.send('deleteFolder', parentKey, grandParent);
        })
        .catch(function() {
          // cancel - noop
        });
    },
  },
});
