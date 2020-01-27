import Route from '@ember/routing/route';
import { get } from '@ember/object';
import { env } from 'consul-ui/env';
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
                Namespace: get(item, 'Namespace'),
                AccessorID: get(item, 'AccessorID'),
                SecretID: secret,
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
                // TODO: Ideally we wouldn't need to use env() at a route level
                // transitionTo should probably remove it instead if NSPACES aren't enabled
                if (env('CONSUL_NSPACES_ENABLED') && get(item, 'token.Namespace') !== nspace) {
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
