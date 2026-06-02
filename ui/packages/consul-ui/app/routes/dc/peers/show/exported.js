/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from 'consul-ui/routing/route';

export default class PeersShowExportedRoute extends Route {
  queryParams = {
    search: {
      as: 'filter',
    },
  };
}
