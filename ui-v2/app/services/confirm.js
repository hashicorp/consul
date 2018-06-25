import Service from '@ember/service';
import confirm from 'consul-ui/utils/confirm';
// TODO: This can go?
export default Service.extend({
  execute: function(message) {
    return confirm(message);
  },
});
