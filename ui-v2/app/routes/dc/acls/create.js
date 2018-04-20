import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get, set } from '@ember/object';

import WithFeedback from 'consul-ui/mixins/with-feedback';
export default Route.extend(WithFeedback, {
  templateName: 'dc/acls/edit',
  repo: service('acls'),
  model: function(params) {
    const item = get(this, 'repo').create();
    set(item, 'Datacenter', this.modelFor('dc').dc.Name);
    return hash({
      isLoading: false,
      create: true,
      item: item,
      types: ['management', 'client'],
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    create: function(item) {
      get(this, 'feedback').execute(
        () => {
          return get(this, 'repo')
            .persist(item)
            .then(item => {
              return this.transitionTo('dc.acls.edit', get(item, 'ID'));
            });
        },
        `Your ACL token has been added.`,
        `There was an error adding your ACL token.`
      );
    },
    cancel: function(item) {
      this.transitionTo('dc.acls');
    },
  },
});
