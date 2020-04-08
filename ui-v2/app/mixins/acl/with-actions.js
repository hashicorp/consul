import Mixin from '@ember/object/mixin';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';
import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default Mixin.create(WithBlockingActions, {
  settings: service('settings'),
  actions: {
    use: function(item) {
      return this.settings.persist({
        token: {
          Namespace: 'default',
          AccessorID: null,
          SecretID: get(item, 'ID'),
        },
      });
    },
    logout: function(item) {
      return this.settings.delete('token');
    },
    clone: function(item) {
      return this.feedback.execute(() => {
        return this.repo.clone(item).then(item => {
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
