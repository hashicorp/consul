/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { diff, filters } from './utils';

export default class SearchBar extends Component {
  // only show the filter status bar if we have searchproperty filters or
  // normal types of filters, and we are currently filtering by either of those
  get isFiltered() {
    const searchproperty = this.args.filter.searchproperty || { default: [], value: [] };
    return (
      diff(searchproperty.default, searchproperty.value).length > 0 ||
      Object.entries(this.args.filter).some(([key, value]) => {
        return key !== 'searchproperty' && typeof value.value !== 'undefined';
      })
    );
  }

  // convert the object based filters to an array of iterable filters ready for
  // rendering
  get filters() {
    return filters(this.args.filter);
  }

  @action
  removeAllFilters() {
    Object.values(this.args.filter).forEach((value, i) => {
      // put in a little queue to ensure query params are unset properly
      // ideally this would be done outside of the component
      // TODO: Look to see if this can be moved to serializeQueryParam
      // so we we aren't polluting components with queryParam related things
      setTimeout(() => value.change(value.default || []), 1 * i);
    });
  }
}
