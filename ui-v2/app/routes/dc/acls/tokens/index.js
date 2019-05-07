import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';
import WithTokenActions from 'consul-ui/mixins/token/with-actions';
export default Route.extend(WithTokenActions, {
  repo: service('repository/token'),
  settings: service('settings'),
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  beforeModel: function(transition) {
    return get(this, 'settings')
      .findBySlug('token')
      .then(token => {
        // If you have a token set with AccessorID set to null (legacy mode)
        // then rewrite to the old acls
        if (token && get(token, 'AccessorID') === null) {
          // If you return here, you get a TransitionAborted error in the tests only
          // everything works fine either way checking things manually
          this.replaceWith('dc.acls');
        }
      });
  },
  model: function(params) {
    const repo = get(this, 'repo');
    return hash({
      ...repo.status({
        items: repo.findAllByDatacenter(this.modelFor('dc').dc.Name),
      }),
      isLoading: false,
      token: get(this, 'settings').findBySlug('token'),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
