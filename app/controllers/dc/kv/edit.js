import Controller from '@ember/controller';
import { computed } from '@ember/object';
import confirm from 'consul-ui/utils/confirm';
import error from 'consul-ui/utils/error';

export default Controller.extend({
  requestDelete: function(item) {
    confirm('Are you sure you want to delete this key?')
      .then(confirmed => {
        if (confirmed) {
          return this.send('delete', item);
        }
      })
      .catch(error);
  },
  requestInvalidateSession: function(item) {
    confirm('Are you sure you want to invalidate this session?')
      .then(confirmed => {
        if (confirmed) {
          return this.send('invalidateSession', item);
        }
      })
      .catch(error);
  },
});
