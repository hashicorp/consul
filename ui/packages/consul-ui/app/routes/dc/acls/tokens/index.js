import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';
import { get } from '@ember/object';
import WithTokenActions from 'consul-ui/mixins/token/with-actions';

export default class IndexRoute extends Route.extend(WithTokenActions) {
  @service('repository/token') repo;
  @service('settings') settings;

  queryParams = {
    sortBy: 'sort',
    kind: 'kind',
    searchproperty: {
      as: 'searchproperty',
      empty: [['AccessorID', 'Description', 'Role', 'Policy']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };

  beforeModel(transition) {
    return this.settings.findBySlug('token').then(token => {
      // If you have a token set with AccessorID set to null (legacy mode)
      // then rewrite to the old acls
      if (token && get(token, 'AccessorID') === null) {
        // If you return here, you get a TransitionAborted error in the tests only
        // everything works fine either way checking things manually
        this.replaceWith('dc.acls');
      }
    });
  }

  model(params) {
    return hash({
      ...this.repo.status({
        items: this.repo.findAllByDatacenter(
          this.modelFor('dc').dc.Name,
          this.modelFor('nspace').nspace.substr(1)
        ),
      }),
      nspace: this.modelFor('nspace').nspace.substr(1),
      token: this.settings.findBySlug('token'),
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
