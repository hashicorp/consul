import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

// TODO: This route and the create Route need merging somehow
export default Route.extend({
  repo: service('repository/intention'),
  model: function(params, transition) {
    const from = get(transition, 'from');
    this.history = [];
    if (from && get(from, 'name') === 'dc.services.show.intentions') {
      this.history.push({
        key: get(from, 'name'),
        value: get(from, 'parent.params.name'),
      });
    }

    const dc = this.modelFor('dc').dc.Name;
    // We load all of your services that you are able to see here
    // as even if it doesn't exist in the namespace you are targetting
    // you may want to add it after you've added the intention
    const nspace = '*';
    return hash({
      isLoading: false,
      dc: dc,
      nspace: nspace,
      slug: params.id,
      item: this.repo.findBySlug(params.id, dc, nspace),
      history: this.history,
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
