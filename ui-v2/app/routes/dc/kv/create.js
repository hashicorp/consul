import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get, set } from '@ember/object';
import WithKvActions from 'consul-ui/mixins/kv/with-actions';
export default Route.extend(WithKvActions, {
  templateName: 'dc/kv/edit',
  repo: service('kv'),
  beforeModel: function(transition) {
    const url = get(transition, 'intent.url');
    const search = '/create/';
    if (url && url.endsWith(search)) {
      return this.replaceWith('dc.kv.folder', this.paramsFor(this.routeName).key + search);
    }
    get(this, 'repo').invalidate();
  },
  model: function(params) {
    const key = params.key || '/';
    const repo = get(this, 'repo');
    const dc = this.modelFor('dc').dc.Name;
    this.item = repo.create();
    set(this.item, 'Datacenter', dc);
    return hash({
      create: true,
      isLoading: false,
      item: this.item,
      parent: repo.findBySlug(key, dc),
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
  deactivate: function() {
    if (get(this.item, 'isNew')) {
      this.item.destroyRecord();
    }
  },
});
