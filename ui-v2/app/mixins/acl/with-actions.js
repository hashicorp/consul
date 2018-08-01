import Mixin from '@ember/object/mixin';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';
import WithFeedback from 'consul-ui/mixins/with-feedback';

export default Mixin.create(WithFeedback, {
  settings: service('settings'),
  actions: {
    create: function(item) {
      get(this, 'feedback').execute(() => {
        return get(this, 'repo')
          .persist(item)
          .then(item => {
            return this.transitionTo('dc.acls');
          });
      }, 'create');
    },
    update: function(item) {
      get(this, 'feedback').execute(() => {
        return get(this, 'repo')
          .persist(item)
          .then(() => {
            return this.transitionTo('dc.acls');
          });
      }, 'update');
    },
    delete: function(item) {
      get(this, 'feedback').execute(() => {
        return (
          get(this, 'repo')
            // ember-changeset doesn't support `get`
            // and `data` returns an object not a model
            .remove(item)
            .then(() => {
              switch (this.routeName) {
                case 'dc.acls.index':
                  return this.refresh();
                default:
                  return this.transitionTo('dc.acls');
              }
            })
        );
      }, 'delete');
    },
    cancel: function(item) {
      this.transitionTo('dc.acls');
    },
    use: function(item) {
      get(this, 'feedback').execute(() => {
        return get(this, 'settings')
          .persist({ token: get(item, 'ID') })
          .then(() => {
            this.transitionTo('dc.services');
          });
      }, 'use');
    },
    clone: function(item) {
      get(this, 'feedback').execute(() => {
        return get(this, 'repo')
          .clone(item)
          .then(item => {
            switch (this.routeName) {
              case 'dc.acls.index':
                return this.refresh();
              default:
                return this.transitionTo('dc.acls');
            }
          });
      }, 'clone');
    },
  },
});
