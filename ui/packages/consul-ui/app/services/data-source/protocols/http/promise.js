import Service from '@ember/service';
import { once } from 'consul-ui/utils/dom/event-source';

export default Service.extend({
  source: function(find, configuration) {
    return once(find, configuration);
  },
});
