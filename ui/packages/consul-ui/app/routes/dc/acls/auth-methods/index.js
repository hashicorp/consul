/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Route from 'consul-ui/routing/route';

export default class IndexRoute extends Route {
  queryParams = {
    sortBy: 'sort',
    source: 'source',
    kind: 'kind',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name', 'DisplayName']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
