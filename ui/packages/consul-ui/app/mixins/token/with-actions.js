import Mixin from '@ember/object/mixin';
import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';

export default Mixin.create(WithBlockingActions, {
  settings: service('settings'),
  actions: {
    use: function(item) {
      return this.repo
        .findBySlug({
          dc: this.modelFor('dc').dc.Name,
          ns: get(item, 'Namespace'),
          id: get(item, 'AccessorID'),
        })
        .then(item => {
          return this.settings.persist({
            token: {
              AccessorID: get(item, 'AccessorID'),
              SecretID: get(item, 'SecretID'),
              Namespace: get(item, 'Namespace'),
            },
          });
        });
    },
    logout: function(item) {
      return this.settings.delete('token');
    },
    clone: function(item) {
      let cloned;
      return this.feedback.execute(() => {
        return this.repo
          .clone(item)
          .then(item => {
            cloned = item;
            // cloning is similar to delete in that
            // if you clone from the listing page, stay on the listing page
            // whereas if you clone from another token, take me back to the listing page
            // so I can see it
            return this.afterDelete(...arguments);
          })
          .then(function() {
            return cloned;
          });
      }, 'clone');
    },
  },
});
