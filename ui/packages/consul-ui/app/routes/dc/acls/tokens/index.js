/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default class IndexRoute extends Route.extend(WithBlockingActions) {
  @service('repository/token') repo;
  queryParams = {
    sortBy: 'sort',
    kind: 'kind',
    searchproperty: {
      as: 'searchproperty',
      empty: [['AccessorID', 'Description', 'Role', 'Policy']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
