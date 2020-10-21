import Service from '@ember/service';
import atob from 'consul-ui/utils/atob';
export default Service.extend({
  execute: function() {
    return atob(...arguments);
  },
});
