import Controller from '@ember/controller';
import { get } from '@ember/object';
import WithFiltering from 'consul-ui/mixins/with-filtering';
import confirm from 'consul-ui/utils/confirm';
import error from 'consul-ui/utils/error';

export default Controller.extend(WithFiltering, {
  tabs: ['Health Checks', 'Services', 'Round Trip Time', 'Lock Sessions'],
  filter: function(item, { s = '', status = '' }) {
    return (
      get(item, 'Service')
        .toLowerCase()
        .indexOf(s.toLowerCase()) === 0
    );
  },
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
