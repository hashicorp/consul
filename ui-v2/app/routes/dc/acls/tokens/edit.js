import SingleRoute from 'consul-ui/routing/single';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { set, get } from '@ember/object';

import WithTokenActions from 'consul-ui/mixins/token/with-actions';
const updateObject = function(arr, item, prop) {
  const id = get(item, prop);
  const i = arr.reduce(function(prev, item, i) {
    if (typeof prev === 'number') {
      return prev;
    }
    if (get(item, prop) === id) {
      return i;
    }
    return;
  }, null);
  const current = arr.objectAt(i);
  Object.keys(item.get('data')).forEach(function(prop) {
    set(current, prop, get(item, prop));
  });
};

export default SingleRoute.extend(WithTokenActions, {
  repo: service('tokens'),
  policiesRepo: service('policies'),
  datacenterRepo: service('dc'),
  model: function(params, transition) {
    const dc = this.modelFor('dc').dc.Name;
    return this._super(...arguments).then(model => {
      return hash({
        ...model,
        ...{
          items: get(this, 'policiesRepo').findAllByDatacenter(dc),
          datacenters: get(this, 'datacenterRepo').findAll(),
          policy: this.getEmptyPolicy(),
        },
      });
    });
  },
  getEmptyPolicy: function() {
    const dc = this.modelFor('dc').dc.Name;
    //TODO: Check to make sure we actually scope to a DC?
    return get(this, 'policiesRepo').create({ Datacenter: dc });
  },
  actions: {
    loadPolicy: function(item) {
      if (!get(item, 'Rules')) {
        const dc = this.modelFor('dc').dc.Name;
        const repo = get(this, 'policiesRepo');
        const slug = get(item, repo.getSlugKey());
        const policies = get(this.controller, 'item.Policies');
        repo.findBySlug(slug, dc).then(item => {
          updateObject(policies, item, repo.getSlugKey());
        });
      }
    },
    removePolicy: function(item, policy) {
      set(item, 'Policies', get(item, 'Policies').without(policy));
    },
    addPolicy: function(item) {
      set(item, 'CreateTime', new Date().getTime());
      if (!get(item, 'ID')) {
        set(item, 'ID', get(item, 'CreateTime'));
      }
      get(this.controller, 'item.Policies').pushObject(item);
      return item;
    },
    createPolicy: function(item, cb) {
      if (typeof cb === 'function') {
        cb();
      }
      this.send('addPolicy', item);
      set(this.controller, 'policy', this.getEmptyPolicy());
      setTimeout(() => {
        get(this, 'policiesRepo')
          .persist(item)
          .then(item => {
            console.log(item.get('data'));
          });
      }, 1000);
    },
  },
});
