import { inject as service } from '@ember/service';
import { alias } from '@ember/object/computed';
import Controller from '@ember/controller';
import { get, action } from '@ember/object';
export default class ShowController extends Controller {
  @service('dom')
  dom;

  @service('flashMessages')
  notify;

  @alias('items.firstObject')
  item;

  @action
  error(e) {
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
      [
        e.target,
        this.intentions,
        this.chain,
        this.proxies,
        this.gatewayServices,
        this.topology,
      ].forEach(function(item) {
        if (item && typeof item.close === 'function') {
          item.close();
        }
      });
    }
  }
}
