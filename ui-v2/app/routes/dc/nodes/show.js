import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get, set } from '@ember/object';

import distance from 'consul-ui/utils/distance';
import tomographyFactory from 'consul-ui/utils/tomography';
import WithFeedback from 'consul-ui/mixins/with-feedback';

const tomography = tomographyFactory(distance);

export default Route.extend(WithFeedback, {
  repo: service('nodes'),
  sessionRepo: service('session'),
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
  },
  model: function(params) {
    const dc = this.modelFor('dc').dc.Name;
    const repo = get(this, 'repo');
    const sessionRepo = get(this, 'sessionRepo');
    return hash({
      item: repo.findBySlug(params.name, dc),
    }).then(function(model) {
      // TODO: Consider loading this after initial page load
      const coordinates = get(model.item, 'Coordinates');
      return hash({
        ...model,
        ...{
          tomography:
            get(coordinates, 'length') > 1
              ? tomography(params.name, coordinates.map(item => get(item, 'data')))
              : null,
          items: get(model.item, 'Services'),
          sessions: sessionRepo.findByNode(get(model.item, 'Node'), dc),
        },
      });
    });
  },
  setupController: function(controller, model) {
    this._super(...arguments);
    controller.setProperties(model);
  },
  actions: {
    invalidateSession: function(item) {
      const dc = this.modelFor('dc').dc.Name;
      const controller = this.controller;
      const repo = get(this, 'sessionRepo');
      get(this, 'feedback').execute(
        () => {
          const node = get(item, 'Node');
          return repo.remove(item).then(() => {
            return repo.findByNode(node, dc).then(function(sessions) {
              set(controller, 'sessions', sessions);
            });
          });
        },
        `The session was invalidated.`,
        `There was an error invalidating the session.`
      );
    },
  },
});
