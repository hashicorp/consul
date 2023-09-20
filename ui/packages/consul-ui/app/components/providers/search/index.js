/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';

export default class SearchProvider extends Component {
  // custom base route / router abstraction is doing weird things
  get _search() {
    return this.args.search || '';
  }

  get items() {
    const { items, searchProperties } = this.args;
    const { _search: search } = this;

    if (search.length > 0) {
      return items.filter((item) => {
        const matchesInSearchProperties = searchProperties.reduce((acc, searchProperty) => {
          const match = item[searchProperty].indexOf(search) !== -1;
          if (match) {
            return [...acc, match];
          } else {
            return acc;
          }
        }, []);
        return matchesInSearchProperties.length > 0;
      });
    } else {
      return items;
    }
  }

  get data() {
    const { items } = this;
    return {
      items,
    };
  }
}
