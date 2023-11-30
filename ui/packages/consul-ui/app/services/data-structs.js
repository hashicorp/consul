/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service from '@ember/service';

import createGraph from 'ngraph.graph';

export default class DataStructsService extends Service {
  graph() {
    return createGraph();
  }
}
