/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Service from '@ember/service';

import createGraph from 'ngraph.graph';

export default class DataStructsService extends Service {
  graph() {
    return createGraph();
  }
}
