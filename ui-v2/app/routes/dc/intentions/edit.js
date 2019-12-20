import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

import WithIntentionActions from 'consul-ui/mixins/intention/with-actions';

// TODO: This route and the create Route need merging somehow
export default Route.extend(WithIntentionActions, {
  repo: service('repository/intention'),
  servicesRepo: service('repository/service'),
  nspacesRepo: service('repository/nspace/disabled'),
  model: function(params) {
    const dc = this.modelFor('dc').dc.Name;
    // We load all of your services that you are able to see here
    // as even if it doesn't exist in the namespace you are targetting
    // you may want to add it after you've added the intention
    const nspace = '*';
    return hash({
      isLoading: false,
      item: this.repo.findBySlug(params.id, dc, nspace),
      services: this.servicesRepo.findAllByDatacenter(dc, nspace),
      nspaces: this.nspacesRepo.findAll(),
    }).then(function(model) {
      return {
        ...model,
        ...{
          services: [{ Name: '*' }].concat(
            model.services.toArray().filter(item => get(item, 'Kind') !== 'connect-proxy')
          ),
          nspaces: [{ Name: '*' }].concat(model.nspaces.toArray()),
        },
      };
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
