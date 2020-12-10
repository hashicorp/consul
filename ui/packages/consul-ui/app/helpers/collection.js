import Helper from '@ember/component/helper';

import { Collection as Service } from 'consul-ui/models/service';
import { Collection as ServiceInstance } from 'consul-ui/models/service-instance';

const collections = {
  service: Service,
  'service-instance': ServiceInstance,
};
class EmptyCollection {}
export default class CollectionHelper extends Helper {
  compute([collection, str], hash) {
    if (collection.length > 0) {
      return new Service(collection);
      // TODO: Looksee if theres ever going to be a public way to get this
      const modelName = collection[0]._internalModel.modelName;
      const Collection = collections[modelName];
      return new Collection(collection);
    } else {
      return new EmptyCollection();
    }
  }
}
