/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from 'consul-ui/routing/route';

export default class PeersShowExportedRoute extends Route {
  queryParams = {
    search: {
      as: 'filter',
    },
  };  // Object format in route with configuration
}