import Service from '@ember/service';
import btoa from 'consul-ui/utils/btoa';
export default Service.extend({
  execute: function() {
    return btoa(...arguments);
  },
});
