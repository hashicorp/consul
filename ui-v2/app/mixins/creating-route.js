import Mixin from '@ember/object/mixin';
import { get } from '@ember/object';

/**
 * Used for create-type Routes
 *
 * 'repo' is standardized across the app
 * 'item' is standardized across the app
 *  they could be replaced with `getRepo` and `getItem`
 */
export default Mixin.create({
  beforeModel: function() {
    get(this, 'repo').invalidate();
  },
  deactivate: function() {
    const item = get(this.controller, 'item.data');
    if (get(item, 'isNew')) {
      item.destroyRecord();
    }
  },
});
