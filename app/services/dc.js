import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';

export default Service.extend({
  store: service('store'),
  findAll: function() {
    return get(this, 'store').findAll('dc');
    // .then(function(items) {
    //   return items.map(function(item) {
    //     return item.get('Name');
    //   });
    // });
  },
});
