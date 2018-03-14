import Controller from '@ember/controller';
import confirm from 'consul-ui/utils/confirm';

export default Controller.extend({
  actions: {
    requestUse: function(acl) {
      confirm('Are you sure you want to use this token for your session?')
        .then(() => {
          return this.send('use', acl);
        })
        .catch(function(e) {
          // cancel - noop
        });
    },
    requestDelete: function(acl) {
      confirm('Are you sure you want to delete this token?')
        .then(() => {
          return this.send('delete', acl);
        })
        .catch(function(e) {
          // cancel - noop
        });
    },
  },
});
