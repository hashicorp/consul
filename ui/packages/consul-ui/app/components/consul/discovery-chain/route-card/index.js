/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';
import { get } from '@ember/object';

export default class RouteCard extends Component {
  get path() {
    return Object.entries(get(this.args.item, 'Definition.Match.HTTP') || {}).reduce(
      function (prev, [key, value]) {
        if (key.toLowerCase().startsWith('path')) {
          return {
            type: key.replace('Path', ''),
            value: value,
          };
        }
        return prev;
      },
      {
        type: 'Prefix',
        value: '/',
      }
    );
  }
}
