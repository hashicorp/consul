import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';
import WithKvActions from 'consul-ui/mixins/kv/with-actions';

export default Route.extend(WithKvActions, {
  templateName: 'dc/kv/edit',
  repo: service('repository/kv'),
  beforeModel: function() {
    this.repo.invalidate();
  },
  model: function(params) {
    const key = params.key || '/';
    const dc = this.modelFor('dc').dc.Name;
    const nspace = this.modelFor('nspace').nspace.substr(1);
    this.item = this.repo.create({
      Datacenter: dc,
      Namespace: nspace,
    });
    return hash({
      create: true,
      isLoading: false,
      item: this.item,
      parent: this.repo.findBySlug(key, dc, nspace),
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
