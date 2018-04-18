import Controller from '@ember/controller';
import { get, set } from '@ember/object';
import confirm from 'consul-ui/utils/confirm';
import error from 'consul-ui/utils/error';
const btoa = window.btoa;
export default Controller.extend({
  json: false,
  actions: {
    change: function(e) {
      const target = e.target || { name: 'value', value: e };
      switch (target.name) {
        case 'basename':
          set(this, 'item.Key', `${get(this, 'parentKey')}${target.value}`);
          break;
        case 'json':
          set(this, 'json', !get(this, 'json'));
          break;
        case 'value':
          set(this, 'item.Value', btoa(target.value));
          break;
      }
    },
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
  },
});
