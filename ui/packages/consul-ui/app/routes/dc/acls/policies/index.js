/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default class IndexRoute extends Route.extend(WithBlockingActions) {
  @service('repository/policy') repo;
  queryParams = {
    sortBy: 'sort',
    datacenter: {
      as: 'dc',
    },
    kind: 'kind',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name', 'Description']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
