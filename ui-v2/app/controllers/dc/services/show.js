import Controller from '@ember/controller';
import { get } from '@ember/object';
import { alias } from '@ember/object/computed';
import { inject as service } from '@ember/service';
export default Controller.extend({
  dom: service('dom'),
  notify: service('flashMessages'),
  item: alias('items.firstObject'),
  actions: {
    error: function(e) {
      if (e.target.readyState === 1) {
        // OPEN
        if (get(e, 'error.errors.firstObject.status') === '404') {
          this.notify.add({
            destroyOnClick: false,
            sticky: true,
            type: 'warning',
            action: 'update',
          });
        }
        [e.target, this.intentions, this.chain, this.proxies, this.gatewayServices].forEach(
          function(item) {
            if (item && typeof item.close === 'function') {
              item.close();
            }
          }
        );
      }
    },
  },
});
