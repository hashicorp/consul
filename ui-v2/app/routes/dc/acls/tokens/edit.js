import SingleRoute from 'consul-ui/routing/single';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { set, get } from '@ember/object';
import updateArrayObject from 'consul-ui/utils/update-array-object';

import WithTokenActions from 'consul-ui/mixins/token/with-actions';

export default SingleRoute.extend(WithTokenActions, {
  repo: service('tokens'),
  policiesRepo: service('policies'),
  datacenterRepo: service('dc'),
  settings: service('settings'),
  model: function(params, transition) {
    const dc = this.modelFor('dc').dc.Name;
    const policiesRepo = get(this, 'policiesRepo');
    return this._super(...arguments).then(model => {
      return hash({
        ...model,
        ...{
          // TODO: I only need these to create a new policy
          datacenters: get(this, 'datacenterRepo').findAll(),
          policy: this.getEmptyPolicy(),
          token: get(this, 'settings').findBySlug('token'),
        },
        ...policiesRepo.status({
          items: policiesRepo.findAllByDatacenter(dc),
        }),
      });
    });
  },
  getEmptyPolicy: function() {
    const dc = this.modelFor('dc').dc.Name;
    // TODO: Check to make sure we actually scope to a DC?
    return get(this, 'policiesRepo').create({ Datacenter: dc });
  },
  actions: {
    // TODO: Some of this could potentially be moved to the
    // repo services

    // triggered when an accordian pane is opened
    loadPolicy: function(item) {
      if (!get(item, 'Rules')) {
        const dc = this.modelFor('dc').dc.Name;
        const repo = get(this, 'policiesRepo');
        const slug = get(item, repo.getSlugKey());
        const policies = get(this.controller, 'item.Policies');
        repo.findBySlug(slug, dc).then(item => {
          updateArrayObject(policies, item, repo.getSlugKey());
        });
      }
    },
    // removing, not deleting, a policy associated with this token
    removePolicy: function(item, policy) {
      set(item, 'Policies', get(item, 'Policies').without(policy));
    },
    // adding an already existing policy
    // also called after a createPolicy
    addPolicy: function(item, now = new Date().getTime()) {
      // abuse CreateTime to get the ordering so the most recently
      // added policy is at the top
      // CreateTime is never sent back to the server
      set(item, 'CreateTime', now);
      if (!get(item, 'ID')) {
        set(item, 'ID', get(item, 'CreateTime'));
      }
      get(this.controller.item, 'Policies').pushObject(item);
      return item;
    },
    // from modal
    clearPolicy: function(cb) {
      set(this.controller, 'policy', this.getEmptyPolicy());
      if (typeof cb === 'function') {
        cb();
      }
    },
    createPolicy: function(item, cb) {
      const repo = get(this, 'policiesRepo');
      const policies = get(this.controller, 'item.Policies');
      this.send('clearPolicy', cb);
      get(this, 'policiesRepo')
        .persist(item)
        .then(item => {
          try {
            this.send('addPolicy', item);
          } catch (e) {}
        });
    },
  },
});
