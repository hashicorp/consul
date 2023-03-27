/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

import chart from './chart.xstate';

export default class TokenSource extends Component {
  @tracked provider;
  @tracked jwt;

  constructor() {
    super(...arguments);
    this.chart = chart;
  }

  @action
  isSecret() {
    return this.args.type === 'secret';
  }

  @action
  change(e) {
    e.data.toJSON = function () {
      return {
        AccessorID: this.AccessorID,
        // TODO: In the past we've always ignored the SecretID returned
        // from the server and used what the user typed in instead, now
        // as we don't know the SecretID when we use SSO we use the SecretID
        // in the response
        SecretID: this.SecretID,
        Namespace: this.Namespace,
        Partition: this.Partition,
        ...{
          AuthMethod: typeof this.AuthMethod !== 'undefined' ? this.AuthMethod : undefined,
          // TODO: We should be able to only set namespaces if they are enabled
          // but we might be testing for nspaces everywhere
          // Namespace: typeof this.Namespace !== 'undefined' ? this.Namespace : undefined
        },
      };
    };
    // TODO: We should probably put the component into idle state
    if (typeof this.args.onchange === 'function') {
      this.args.onchange(e);
    }
  }
}
