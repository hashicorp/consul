import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';

import $ from 'jquery';
import { Promise } from 'rsvp';
const request = function() {
  return new Promise((resolve, reject) => {
    $.ajax(...arguments)
      .then(function(res) {
        resolve(res);
        return res;
      })
      .fail(function(e) {
        reject(e);
        return e;
      });
  });
}
export default Service.extend({
  store: service('store'),
  findAllByDatacenter: function(dc) {
    return this.get('store').query('node', { dc: dc });
  },
  findBySlug: function(slug, dc) {
    return this.get('store').queryRecord('node', {
      id: slug,
      dc: dc,
    }).then(
      (node) => {
        return this.findAllCoordinatesByDatacenter(dc).then(
          function(res) {
            node.coordinates = res;
            return node;
          }
        );

      }
    );
  },
  findAllCoordinatesByDatacenter: function(dc) {
    console.warn('TODO: not ember-data');
    return request('/v1/coordinate/nodes', dc);
  },
});
