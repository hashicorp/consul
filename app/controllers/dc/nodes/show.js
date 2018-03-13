import Controller from '@ember/controller';
import confirm from 'consul-ui/utils/confirm';

export default Controller.extend({
  actions: {
    requestInvalidateSession: function(session) {
      confirm('Are you sure you want to invalidate this session?')
        .then(() => {
          return this.send('invalidateSession', session);
        })
        .catch(function() {
          // cancelled - noop
        });
    },
  },
});
