import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';

export default Route.extend({
  repo: service('repository/dc'),
  model: function(params) {
    return hash({
      item: this.repo.getActive(),
      nspace: params.nspace,
    });
  },

  /**
   * We need to redirect if someone doesn't specify the section they want,
   * but not redirect if the 'section' is specified
   * (i.e. /dc-1/ vs /dc-1/services).
   *
   * If the target route of the transition is `nspace.index`, it means that
   * someone didn't specify a section and thus we forward them on to a
   * default `.services` subroute.  The specific services route we target
   * depends on whether or not a namespace was specified.
   *
   */
  afterModel(model, transition) {
    if (transition.to.name === 'nspace.index') {
      if (model.nspace.startsWith('~')) {
        this.transitionTo('nspace.dc.services', model.nspace, model.item.Name);
      } else {
        this.transitionTo('dc.services', model.nspace);
      }
    }
  },
});
