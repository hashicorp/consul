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
      return this.repo
        .self(secret, dc)
        .then(item => {
          return this.settings.persist({
            token: {
              Namespace: get(item, 'Namespace'),
              AccessorID: get(item, 'AccessorID'),
              SecretID: secret,
            },
          });
        })
        .catch(e => {
          this.feedback.execute(
            () => {
              return Promise.resolve();
            },
            'authorize',
            function(type, e) {
              return 'error';
            },
            {}
          );
        });
    },
  },
});
