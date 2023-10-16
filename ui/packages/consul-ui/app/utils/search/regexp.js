/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import PredicateSearch from './predicate';

export default class RegExpSearch extends PredicateSearch {
  predicate(s) {
    let regex;
    try {
      regex = new RegExp(s, 'i');
    } catch (e) {
      // Return a predicate that excludes everything; most likely due to an
      // eager search of an incomplete regex
      return () => false;
    }
    return (item) => regex.test(item);
  }
}
