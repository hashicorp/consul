import Route from '@ember/routing/route';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';
import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';
export default Route.extend(WithBlockingActions, {
  settings: service('settings'),
  feedback: service('feedback'),
  repo: service('repository/token'),
  actions: {
    authorize: function(secret) {
      const dc = this.modelFor('dc').dc.Name;
      return get(this, 'feedback').execute(() => {
        return get(this, 'repo')
          .self(secret, dc)
          .then(item => {
            get(this, 'settings')
              .persist({
                token: {
                  AccessorID: get(item, 'AccessorID'),
                  SecretID: secret,
                },
              })
              .then(item => {
                // a null AccessorID means we are in legacy mode
                // take the user to the legacy acls
                // otherwise just refresh the page
                if (get(item, 'token.AccessorID') === null) {
                  return this.transitionTo('dc.acls');
                } else {
                  this.refresh();
                }
              });
          });
      }, 'authorize');
    },
  },
});
