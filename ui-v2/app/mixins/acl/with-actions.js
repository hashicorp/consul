import Mixin from '@ember/object/mixin';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';
import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default Mixin.create(WithBlockingActions, {
  settings: service('settings'),
  actions: {
    use: function(item) {
      return get(this, 'feedback').execute(() => {
        // old style legacy ACLs don't have AccessorIDs
        // therefore set it to null, this way the frontend knows
        // to use legacy ACLs
        return get(this, 'settings')
          .persist({
            token: {
              AccessorID: null,
              SecretID: get(item, 'ID'),
            },
          })
          .then(() => {
            return this.transitionTo('dc.services');
          });
      }, 'use');
    },
    // TODO: This is also used in tokens, probably an opportunity to dry this out
    logout: function(item) {
      return get(this, 'feedback').execute(() => {
        return get(this, 'settings')
          .delete('token')
          .then(() => {
            // in this case we don't do the same as delete as we want to go to the new
            // dc.acls.tokens page. If we get there via the dc.acls redirect/rewrite
            // then we lose the flash message
            return this.transitionTo('dc.acls.tokens');
          });
      }, 'logout');
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
