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
    let search = '/create/';
    if (url && url.endsWith(search)) {
      const key = this.paramsFor(this.routeName).key || '';
      if (key === '') {
        search = 'create/';
      }
      return this.replaceWith('dc.kv.folder', key + search);
    }
    get(this, 'repo').invalidate();
  },
  model: function(params, transition) {
    const key = params.key || '/';
    const repo = get(this, 'repo');
    const dc = this.modelFor('dc').dc.Name;
    this.item = repo.create();
    set(this.item, 'Datacenter', dc);
    return hash({
      create: true,
      isLoading: false,
      item: this.item,
      parent: repo.findBySlug(key, dc).catch(e => {
        if (e.errors && e.errors[0] && e.errors[0].status == '404') {
          const url = get(transition, 'intent.url');
          if (url.endsWith('/create')) {
            this.replaceWith('dc.kv.folder', key + '/create/');
          }
        }
      }),
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
