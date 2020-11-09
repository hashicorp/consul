import { inject as service } from '@ember/service';
import Route from 'consul-ui/routing/route';
import { hash } from 'rsvp';

export default class IndexRoute extends Route {
  @service('data-source/service')
  data;

  queryParams = {
    search: {
      as: 'filter',
      replace: true,
    },
    // temporary support of old style status
    status: {
      as: 'status',
    },
  };

  model(params) {
    let terms = params.s || '';
    // we check for the old style `status` variable here
    // and convert it to the new style filter=status:critical
    let status = params.status;
    if (status) {
      status = `status:${status}`;
      if (terms.indexOf(status) === -1) {
        terms = terms
          .split('\n')
          .concat(status)
          .join('\n')
          .trim();
      }
    }
    const nspace = this.modelFor('nspace').nspace.substr(1);
    const dc = this.modelFor('dc').dc.Name;
    return hash({
      nspace: nspace,
      dc: dc,
      terms: terms !== '' ? terms.split('\n') : [],
      items: this.data.source(uri => uri`/${nspace}/${dc}/services`),
    });
  }

  setupController(controller, model) {
    super.setupController(...arguments);
    controller.setProperties(model);
  }
}
