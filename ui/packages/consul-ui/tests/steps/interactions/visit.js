/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { visit } from '@ember/test-helpers';

export default function (scenario, pages, set, reset) {
  scenario
    .when('I visit the $name page', function (name) {
      reset();
      return set(pages[name]).visit();
    })
    .when('I visit the $name page for the "$id" $model', function (name, id, model) {
      reset();
      return set(pages[name]).visit({
        [model]: id,
      });
    })
    .when('I visit the $name page with the url $url', function (name, url) {
      reset();
      set(pages[name]);
      return visit(url);
    })
    .when(
      ['I visit the $name page for yaml\n$yaml', 'I visit the $name page for json\n$json'],
      function (name, data) {
        const nspace = this.ctx.nspace;
        if (nspace !== '' && typeof nspace !== 'undefined') {
          data.nspace = `~${nspace}`;
        }
        reset();
        // TODO: Consider putting an assertion here for testing the current url
        // do I absolutely definitely need that all the time?
        return set(pages[name]).visit(data);
      }
    )
    .when(
      ['I $method the $name page for yaml\n$yaml', 'I $method the $name page for json\n$json'],
      function (method, name, data) {
        reset();

        return set(pages[name])[method](data);
      }
    );
}
