import Service from '@ember/service';
import confirm from 'consul-ui/utils/confirm';

export default Service.extend({
  execute: function(message) {
    return confirm(message);
  },
});
