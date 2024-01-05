/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Route from 'consul-ui/routing/route';
import { inject as service } from '@ember/service';

import WithBlockingActions from 'consul-ui/mixins/with-blocking-actions';

export default class IndexRoute extends Route.extend(WithBlockingActions) {
  @service('repository/role') repo;
  queryParams = {
    sortBy: 'sort',
    searchproperty: {
      as: 'searchproperty',
      empty: [['Name', 'Description', 'Policy']],
    },
    search: {
      as: 'filter',
      replace: true,
    },
  };
}
