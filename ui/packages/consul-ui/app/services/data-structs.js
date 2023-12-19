import Service from '@ember/service';

import createGraph from 'ngraph.graph';

export default class DataStructsService extends Service {
  graph() {
    return createGraph();
  }
}
