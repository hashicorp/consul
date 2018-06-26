import Service from '@ember/service';
// TODO: This can go?
import error from 'consul-ui/utils/error';
export default Service.extend({
  execute: function(obj) {
    return error(obj);
  },
});
