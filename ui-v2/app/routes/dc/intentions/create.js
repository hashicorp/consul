import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

// TODO: This route and the edit Route need merging somehow
export default Route.extend({
  templateName: 'dc/intentions/edit',
  repo: service('repository/intention'),
  model: function(params) {
    const dc = this.modelFor('dc').dc.Name;
    const nspace = '*';
    this.item = this.repo.create({
      Datacenter: dc,
    });
    return hash({
      create: true,
      isLoading: false,
      dc: dc,
      nspace: nspace,
      slug: params.id,
      item: this.item,
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  deactivate: function() {
    if (get(this.item, 'isNew')) {
      this.item.rollbackAttributes();
    }
  },
});
