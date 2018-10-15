import SingleRoute from 'consul-ui/routing/single';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { set, get } from '@ember/object';
import updateArrayObject from 'consul-ui/utils/update-array-object';

import WithTokenActions from 'consul-ui/mixins/token/with-actions';

const ERROR_PARSE_RULES = 'Failed to parse ACL rules';
const ERROR_NAME_EXISTS = 'Invalid Policy: A Policy with Name';
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
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
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
    addPolicy: function(item) {
      const controller = get(this, 'controller');
      get(controller, 'item.Policies').pushObject(item);
      return item;
    },
    // from modal
    clearPolicy: function(cb) {
      const controller = get(this, 'controller');
      controller.setProperties({
        policy: this.getEmptyPolicy(),
      });
      if (typeof cb === 'function') {
        cb();
      }
    },
    createPolicy: function(item, cb) {
      get(this, 'policiesRepo')
        .persist(item)
        .then(item => {
          try {
            this.send('addPolicy', item);
            this.send('clearPolicy', cb);
          } catch (e) {
            // continue
          }
        })
        .catch(e => {
          const error = e.errors[0];
          let prop;
          let message = error.detail;
          switch (true) {
            case message.indexOf(ERROR_PARSE_RULES) === 0:
              prop = 'Rules';
              message = error.detail;
              break;
            case message.indexOf(ERROR_NAME_EXISTS) === 0:
              prop = 'Name';
              message = message.substr(ERROR_NAME_EXISTS.indexOf(':') + 1);
              break;
          }
          if (prop) {
            item.addError(prop, message);
          }
        });
    },
  },
});
