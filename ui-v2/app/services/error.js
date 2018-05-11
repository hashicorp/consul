import Service from '@ember/service';

import error from 'consul-ui/utils/error';
export default Service.extend({
  execute: function(obj) {
    return error(obj);
  },
});
