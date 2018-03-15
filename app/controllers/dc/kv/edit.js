import Controller from '@ember/controller';
import { computed } from '@ember/object';
import confirm from 'consul-ui/utils/confirm';
import error from 'consul-ui/utils/error';

export default Controller.extend({
  isLockedOrLoading: computed.or('isLoading', 'isLocked'),
  // this is skipped for now to replicate current UI
  // change the action in the view to requestDelete to enable
  requestDelete: function(item) {
    confirm('Are you sure you want to delete this key?')
      .then(confirmed => {
        if (confirmed) {
          return this.send('delete', item);
        }
      })
      .catch(error);
  },
});
