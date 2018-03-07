import Service, { inject as service } from '@ember/service';

export default Service.extend({
  store: service('store'),
  findAll: function() {
    return this.get('store')
      .findAll('dc')
      .then(function(dcs) {
        return dcs.map(function(item) {
          return item.get('Name');
        });
      });
  },
});
