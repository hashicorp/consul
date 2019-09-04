import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get, set } from '@ember/object';
import WithIntentionActions from 'consul-ui/mixins/intention/with-actions';

export default Route.extend(WithIntentionActions, {
  templateName: 'dc/intentions/edit',
  repo: service('repository/intention'),
  servicesRepo: service('repository/service'),
  beforeModel: function() {
    this.repo.invalidate();
  },
  model: function(params) {
    this.item = this.repo.create();
    set(this.item, 'Datacenter', this.modelFor('dc').dc.Name);
    return hash({
      create: true,
      isLoading: false,
      item: this.item,
      items: this.servicesRepo.findAllByDatacenter(this.modelFor('dc').dc.Name),
      intents: ['allow', 'deny'],
    }).then(function(model) {
      return {
        ...model,
        ...{
          items: [{ Name: '*' }].concat(
            model.items.toArray().filter(item => get(item, 'Kind') !== 'connect-proxy')
          ),
        },
      };
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  deactivate: function() {
    if (get(this.item, 'isNew')) {
      this.item.destroyRecord();
    }
  },
});
