import Mixin from '@ember/object/mixin';
import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

import { INTERNAL_SERVER_ERROR as HTTP_INTERNAL_SERVER_ERROR } from 'consul-ui/utils/http/status';
export default Mixin.create(WithBlockingActions, {
  errorCreate: function(type, e) {
    if (e && e.errors && e.errors[0]) {
      const error = e.errors[0];
      if (parseInt(error.status) === HTTP_INTERNAL_SERVER_ERROR) {
        if (error.detail.indexOf('duplicate intention found:') === 0) {
          return 'exists';
        }
      }
    }
    return type;
  },
});
