import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';
import isFolder from 'consul-ui/utils/isFolder';
import WithKvActions from 'consul-ui/mixins/kv/with-actions';
/**
 * Certain KV routes container extra `beforeModel` hooks to hedge for the
 * fact that the `consul` binary has some 301 redirects to remove
 * trailing slashes, whereas when testing/developing in ember this isn't replicated.
 * There are various 'problems' related to this, including potential problems related
 * to `create` and `edit` endpoints which could cause issues if someone has KV's
 * called `create` or `edit`.
 *
 * Extra `modelFor` hooks are added in the create, edit Routes, plus one below here for
 * more general issues with the binary 301 redirects.
 *
 * Lastly, documentation has been added here only to refer to all routes in KV, as
 * `index` seemed to be the best place to do it without repeating.
 *
 * See https://github.com/hashicorp/consul/pull/4411, https://github.com/hashicorp/consul/pull/4373
 *
 */
export default Route.extend(WithKvActions, {
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  repo: service('kv'),
  beforeModel: function() {
    // we are index or folder, so if the key doesn't have a trailing slash
    // add one to force a fake findBySlug
    const params = this.paramsFor(this.routeName);
    const key = params.key || '/';
    if (!isFolder(key)) {
      return this.replaceWith(this.routeName, key + '/');
    }
  },
  model: function(params) {
    let key = params.key || '/';
    const dc = this.modelFor('dc').dc.Name;
    const repo = get(this, 'repo');
    return hash({
      isLoading: false,
      parent: repo.findBySlug(key, dc),
    }).then(model => {
      return hash({
        ...model,
        ...{
          items: repo.findAllBySlug(get(model.parent, 'Key'), dc).catch(e => {
            this.replaceWith('dc.kv.index');
          }),
        },
      });
    });
  },
  actions: {
    error: function(e) {
      if (e.errors && e.errors[0] && e.errors[0].status == '404') {
        return this.transitionTo('dc.kv.index');
      }
      throw e;
    },
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
