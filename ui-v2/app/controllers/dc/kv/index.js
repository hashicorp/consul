import Controller from '@ember/controller';
import confirm from 'consul-ui/utils/confirm';
import error from 'consul-ui/utils/error';

export default Controller.extend({
  actions: {
    requestDelete: function(item) {
      confirm('Are you sure you want to delete this key?')
        .then(confirmed => {
          if (confirmed) {
            return this.send('delete', item);
          }
        })
        .catch(error);
    },
    requestDeleteFolder: function(parentKey, grandParent) {
      confirm('Are you sure you want to delete this folder?')
        .then(confirmed => {
          if (confirmed) {
            return this.send('deleteFolder', parentKey, grandParent);
          }
        })
        .catch(error);
    },
  },
});
