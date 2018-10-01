import Mixin from '@ember/object/mixin';
import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';

export default Mixin.create(WithBlockingActions, {
  settings: service('settings'),
  actions: {
    use: function(item) {
      return get(this, 'feedback').execute(() => {
        return get(this, 'settings')
          .persist({ token: get(item, 'SecretID') })
          .then(() => {
            return this.transitionTo('dc.acls.tokens');
          });
      }, 'use');
    },
    clone: function(item) {
      return get(this, 'feedback').execute(() => {
        return get(this, 'repo')
          .clone(item)
          .then(item => {
            // cloning is similar to delete in that
            // if you clone from the listing page, stay on the listing page
            // whereas if you clone form another token, take me back to the listing page
            // so I can see it
            return this.afterDelete(...arguments);
          });
      }, 'clone');
    },
  },
});
