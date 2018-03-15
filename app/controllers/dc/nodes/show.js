import Controller from '@ember/controller';
import confirm from 'consul-ui/utils/confirm';
import error from 'consul-ui/utils/error';

export default Controller.extend({
  actions: {
    requestInvalidateSession: function(item) {
      confirm('Are you sure you want to invalidate this session?')
        .then(confirmed => {
          if (confirmed) {
            return this.send('invalidateSession', item);
          }
        })
        .catch(error);
    },
  },
});
