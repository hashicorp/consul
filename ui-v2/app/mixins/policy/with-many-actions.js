import Mixin from '@ember/object/mixin';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import { hash } from 'rsvp';

import updateArrayObject from 'consul-ui/utils/update-array-object';

const ERROR_PARSE_RULES = 'Failed to parse ACL rules';
const ERROR_NAME_EXISTS = 'Invalid Policy: A Policy with Name';

export default Mixin.create({
  policyRepo: service('repository/policy'),
  datacenterRepo: service('repository/dc'),
  getEmptyPolicy: function() {
    const dc = this.modelFor('dc').dc.Name;
    return get(this, 'policyRepo').create({ Datacenter: dc });
  },
  model: function(params, transition) {
    const dc = this.modelFor('dc').dc.Name;
    const policyRepo = get(this, 'policyRepo');
    return this._super(...arguments).then(model => {
      return hash({
        ...model,
        ...{
          // TODO: I only need these to create a new policy
          datacenters: get(this, 'datacenterRepo').findAll(),
          policy: this.getEmptyPolicy(),
          policies: policyRepo.findAllByDatacenter(dc).catch(function(e) {
            switch (get(e, 'errors.firstObject.status')) {
              case '403':
              case '401':
                // do nothing the SingleRoute will have caught it already
                return;
            }
            throw e;
          }),
        },
      });
    });
  },
  actions: {
    // TODO: Some of this could potentially be moved to the repo services
    loadPolicy: function(item, items) {
      const repo = get(this, 'policyRepo');
      const dc = this.modelFor('dc').dc.Name;
      const slug = get(item, repo.getSlugKey());
      repo.findBySlug(slug, dc).then(item => {
        updateArrayObject(items, item, repo.getSlugKey());
      });
    },
    remove: function(item, items) {
      return items.removeObject(item);
    },
    clearPolicy: function() {
      // TODO: I should be able to reset the ember-data object
      // back to it original state?
      // possibly Forms could know how to create
      const controller = get(this, 'controller');
      controller.setProperties({
        policy: this.getEmptyPolicy(),
      });
    },
    createPolicy: function(item, policies, success) {
      get(this, 'policyRepo')
        .persist(item)
        .then(item => {
          set(item, 'CreateTime', new Date().getTime());
          policies.pushObject(item);
          return item;
        })
        .then(function() {
          success();
        })
        .catch(err => {
          if (typeof err.errors !== 'undefined') {
            const error = err.errors[0];
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
          } else {
            throw err;
          }
        });
    },
  },
});
