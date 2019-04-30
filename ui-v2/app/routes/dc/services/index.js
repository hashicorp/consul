import Route from '@ember/routing/route';
import { inject as service } from '@ember/service';
import { hash } from 'rsvp';
import { get } from '@ember/object';

export default Route.extend({
  repo: service('repository/service'),
  queryParams: {
    s: {
      as: 'filter',
      replace: true,
    },
    // temporary support of old style status
    status: {
      as: 'status',
    },
  },
  model: function(params) {
    const repo = get(this, 'repo');
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
    return hash({
      terms: terms !== '' ? terms.split('\n') : [],
      items: repo.findAllByDatacenter(this.modelFor('dc').dc.Name),
    });
  },
  setupController: function(controller, model) {
    controller.setProperties(model);
  },
});
