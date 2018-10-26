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
    // TODO: This is dependent on ember-changeset
    // Change changeset to support ember-data props
    const item = get(this.controller, 'item.data');
    // TODO: Look and see if rollbackAttributes is good here
    if (get(item, 'isNew')) {
      item.destroyRecord();
    }
  },
});
