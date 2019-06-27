import Service from '@ember/service';

import Clipboard from 'npm:clipboard';

export default Service.extend({
  execute: function(trigger) {
    return new Clipboard(trigger);
  },
});
