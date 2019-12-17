import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';
import WithIntentionActions from 'consul-ui/mixins/intention/with-actions';

// TODO: This route and the edit Route need merging somehow
export default Route.extend(WithIntentionActions, {
  templateName: 'dc/intentions/edit',
  repo: service('repository/intention'),
  servicesRepo: service('repository/service'),
  nspacesRepo: service('repository/nspace/disabled'),
  beforeModel: function() {
    this.repo.invalidate();
  },
  model: function(params) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);
    this.item = this.repo.create({
      Datacenter: dc,
    });
    return hash({
      create: true,
      isLoading: false,
      item: this.item,
      services: this.servicesRepo.findAllByDatacenter(dc, nspace),
      nspaces: this.nspacesRepo.findAll(),
    }).then(function(model) {
      return {
        ...model,
        ...{
          services: [{ Name: '*' }].concat(
            model.services.toArray().filter(item => get(item, 'Kind') !== 'connect-proxy')
          ),
          nspaces: [{ Name: '*' }].concat(model.nspaces.toArray()),
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
