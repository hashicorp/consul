/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Helper from '@ember/component/helper';
import { get } from '@ember/object';

import { Collection as Service } from 'consul-ui/models/service';
import { Collection as ServiceInstance } from 'consul-ui/models/service-instance';

const collections = {
  service: Service,
  'service-instance': ServiceInstance,
};
class EmptyCollection {}
export default class CollectionHelper extends Helper {
  compute([collection, str], hash) {
    if (!collection || collection.length === 0) {
      return new EmptyCollection();
    }

    const first = get(collection, 'firstObject') || collection[0];
    const modelName = get(first, 'constructor.modelName') || first?.modelName;
    const Collection = collections[modelName];

    if (!Collection) {
      return new EmptyCollection();
    }

    return new Collection(collection);
  }
}
