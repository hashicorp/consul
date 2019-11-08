import Route from '@ember/routing/route';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';
import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';
export default Route.extend(WithBlockingActions, {
  router: service('router'),
  settings: service('settings'),
  feedback: service('feedback'),
  repo: service('repository/token'),
  actions: {
    authorize: function(secret, nspace) {
      const dc = this.modelFor('dc').dc.Name;
      return this.feedback.execute(() => {
        return this.repo.self(secret, dc).then(item => {
          return this.settings
            .persist({
              token: {
                AccessorID: get(item, 'AccessorID'),
                SecretID: secret,
                Namespace: get(item, 'Namespace'),
              },
            })
            .then(item => {
              // a null AccessorID means we are in legacy mode
              // take the user to the legacy acls
              // otherwise just refresh the page
              if (get(item, 'token.AccessorID') === null) {
                // returning false for a feedback action means even though
                // its successful, please skip this notification and don't display it
                return this.transitionTo('dc.acls').then(function() {
                  return false;
                });
              } else {
                if (get(item, 'token.Namespace') !== nspace) {
                  let routeName = this.router.currentRouteName;
                  if (!routeName.startsWith('nspace')) {
                    routeName = `nspace.${routeName}`;
                  }
                  return this.transitionTo(`${routeName}`, `~${get(item, 'token.Namespace')}`, dc);
                } else {
                  this.refresh();
                }
              }
            });
        });
      }, 'authorize');
    },
  },
});
