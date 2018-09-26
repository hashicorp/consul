import SingleRoute from 'consul-ui/routing/single';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { set, get } from '@ember/object';

import WithTokenActions from 'consul-ui/mixins/token/with-actions';

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
    removePolicy: function(item) {
      const token = get(this.controller, 'item');
      const policies = get(token, 'Policies');
      set(token, 'Policies', policies.without(item));
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
