import Mixin from '@ember/object/mixin';
import { inject as service } from '@ember/service';
import { get, set } from '@ember/object';
import { hash } from 'rsvp';

import updateArrayObject from 'consul-ui/utils/update-array-object';

const ERROR_NAME_EXISTS = 'Invalid Role: A Role with Name';

export default Mixin.create({
  roleRepo: service('repository/role'),
  getEmptyRole: function() {
    const dc = this.modelFor('dc').dc.Name;
    return get(this, 'roleRepo').create({ Datacenter: dc });
  },
  model: function(params, transition) {
    const dc = this.modelFor('dc').dc.Name;
    const repo = get(this, 'roleRepo');
    return this._super(...arguments).then(model => {
      return hash({
        ...model,
        ...{
          role: this.getEmptyRole(),
          roles: repo.findAllByDatacenter(dc).catch(function(e) {
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
    loadRole: function(item, items) {
      const repo = get(this, 'roleRepo');
      const dc = this.modelFor('dc').dc.Name;
      const slug = get(item, repo.getSlugKey());
      repo.findBySlug(slug, dc).then(item => {
        updateArrayObject(items, item, repo.getSlugKey());
      });
    },
    remove: function(item, items) {
      return items.removeObject(item);
    },
    clearRole: function() {
      // TODO: I should be able to reset the ember-data object
      // back to it original state?
      // possibly Forms could know how to create
      const controller = get(this, 'controller');
      controller.setProperties({
        role: this.getEmptyRole(),
      });
    },
    createRole: function(item, items, success) {
      get(this, 'roleRepo')
        .persist(item)
        .then(item => {
          set(item, 'CreateTime', new Date().getTime());
          items.pushObject(item);
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
