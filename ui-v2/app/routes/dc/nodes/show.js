import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default Route.extend(WithBlockingActions, {
  repo: service('repository/node'),
  sessionRepo: service('repository/session'),
  coordinateRepo: service('repository/coordinate'),
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  model: function(params) {
    const dc = this.modelFor('dc').dc.Name;
    const name = params.name;
    return hash({
      item: get(this, 'repo').findBySlug(name, dc),
      tomography: get(this, 'coordinateRepo').findAllByNode(name, dc),
      sessions: get(this, 'sessionRepo').findByNode(name, dc),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
  actions: {
    invalidateSession: function(item) {
      const dc = this.modelFor('dc').dc.Name;
      const controller = this.controller;
      const repo = get(this, 'sessionRepo');
      return get(this, 'feedback').execute(() => {
        const node = get(item, 'Node');
        return repo.remove(item).then(() => {
          return repo.findByNode(node, dc).then(function(sessions) {
            controller.setProperties({
              sessions: sessions,
            });
          });
        });
      }, 'delete');
    },
  },
});
