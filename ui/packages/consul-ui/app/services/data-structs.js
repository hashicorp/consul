import Service from '@ember/service';

import createGraph from 'ngraph.graph';

export default Service.extend({
  graph: function() {
    return createGraph();
  },
});
