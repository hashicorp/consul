import Route from '@ember/routing/route';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';
import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default Route.extend(WithBlockingActions, {
  settings: service('settings'),
  feedback: service('feedback'),
  repo: service('tokens'),
  actions: {
    authorize: function(token) {
      const dc = this.modelFor('dc').dc.Name;
      return get(this, 'feedback').execute(() => {
        return get(this, 'repo')
          .self(token, dc)
          .then(item => {
            get(this, 'settings')
              .persist({
                token: {
                  AccessorID: get(item, 'AccessorID'),
                  SecretID: token,
                },
              })
              .then(() => {
                this.refresh();
              });
          });
      }, 'authorize');
    },
  },
});
