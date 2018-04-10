import Route from '@ember/routing/route';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

import WithFeedback from 'consul-ui/mixins/with-feedback';
export default Route.extend(WithFeedback, {
  templateName: 'dc/acls/edit',
  repo: service('acls'),
  model: function(params) {
    const item = this.get('repo').create();
    item.set('Datacenter', this.modelFor('dc').dc);
    return hash({
      item: item,
      types: ['management', 'client'],
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    create: function(item) {
      this.get('feedback').execute(
        () => {
          return item.save().then(() => {
              this.transitionTo('dc.acls.edit', get(item, 'ID'));
          });
        },
        `Created ${item.get('Name')}`,
        `There was an error using ${item.get('Name')}`
      );
    },
    cancel: function(item) {
      this.transitionTo('dc.acls');
    },
  },
});
