import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithTokenActions from 'consul-ui/mixins/token/with-actions';
const status = function(p, propName = 'items') {
  let authorize;
  let enable;
  return {
    isAuthorized: new Promise(function(resolve) {
      authorize = function(bool) {
        resolve(bool);
      };
    }),
    isEnabled: new Promise(function(resolve) {
      enable = function(bool) {
        resolve(bool);
      };
    }),
    items: p
      .catch(function(e) {
        switch (e.errors[0].status) {
          case '403':
            enable(true);
            break;
          default:
            enable(false);
        }
        authorize(false);
        return;
      })
      .then(function(res) {
        enable(true);
        authorize(true);
        return res;
      }),
  };
};
export default Route.extend(WithTokenActions, {
  repo: service('tokens'),
  settings: service('settings'),
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  model: function(params) {
    return hash({
      ...status(get(this, 'repo').findAllByDatacenter(this.modelFor('dc').dc.Name), 'items'),
      isLoading: false,
      currentAccessorID: get(this, 'settings').findBySlug('accessor_id'),
    }).then(function(model) {
      return hash({
        ...model,
        ...{
          isLegacy: model.items.find(function(item) {
            return get(item, 'Legacy') === true;
          }),
        },
      });
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
});
