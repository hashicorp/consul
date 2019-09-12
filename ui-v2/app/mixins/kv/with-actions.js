import Mixin from '@ember/object/mixin';
import { get, set } from '@ember/object';
import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default Mixin.create(WithBlockingActions, {
  // afterCreate just calls afterUpdate
  afterUpdate: function(item, parent) {
    const key = get(parent, 'Key');
    if (key === '/') {
      return this.transitionTo('dc.kv.index');
    } else {
      return this.transitionTo('dc.kv.folder', key);
    }
  },
  afterDelete: function(item, parent) {
    if (this.routeName === 'dc.kv.folder') {
      return this.refresh();
    }
    return this._super(...arguments);
  },
  actions: {
    invalidateSession: function(item) {
      const controller = this.controller;
      const repo = get(this, 'sessionRepo');
      return get(this, 'feedback').execute(() => {
        return repo.remove(item).then(() => {
          const item = get(controller, 'item');
          set(item, 'Session', null);
          delete item.Session;
          set(controller, 'session', null);
        });
      }, 'deletesession');
    },
  },
});
