/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Component from '@glimmer/component';
import { inject as service } from '@ember/service';
import { action } from '@ember/object';
import { tracked } from '@glimmer/tracking';

const templateRe = /\${([A-Za-z.0-9_-]+)}/g;
export default class RouteComponent extends Component {
  @service('routlet') routlet;
  @service('router') router;
  @service('intl') intl;
  @service('encoder') encoder;

  @tracked _model;

  constructor() {
    super(...arguments);
    this.intlKey = this.encoder.createRegExpEncoder(templateRe, (_) => _);
  }

  get params() {
    return this.routlet.paramsFor(this.args.name);
  }

  get model() {
    if (this._model) {
      return this._model;
    }
    if (this.args.name) {
      const outlet = this.routlet.outletFor(this.args.name);

      if (outlet) {
        return this.routlet.modelFor(outlet.name);
      }
    }
    return undefined;
  }

  @action
  exists(str) {
    return this.routlet.exists(`${this.args.name}.${str}`);
  }

  @action
  t(str, options) {
    if (str.includes('${')) {
      str = this.intlKey(str, options);
    }
    return this.intl.t(`routes.${this.args.name}.${str}`, options);
  }

  @action
  connect() {
    this.routlet.addRoute(this.args.name, this);
  }

  @action
  disconnect() {
    this.routlet.removeRoute(this.args.name, this);
  }
}
