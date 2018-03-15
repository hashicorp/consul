import Controller from '@ember/controller';
import confirm from 'consul-ui/utils/confirm';
import error from 'consul-ui/utils/error';
export default Controller.extend({
  actions: {
    requestUse: function(item) {
      confirm('Are you sure you want to use this token for your session?')
        .then(confirmed => {
          if (confirmed) {
            return this.send('use', item);
          }
        })
        .catch(error);
    },
    requestDelete: function(item) {
      confirm('Are you sure you want to delete this token?')
        .then(confirmed => {
          if (confirmed) {
            return this.send('delete', item);
          }
        })
        .catch(error);
    },
  },
});
