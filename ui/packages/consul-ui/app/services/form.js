/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import Service, { inject as service } from '@ember/service';
import builderFactory from 'consul-ui/utils/form/builder';

import kv from 'consul-ui/forms/kv';
import token from 'consul-ui/forms/token';
import policy from 'consul-ui/forms/policy';
import role from 'consul-ui/forms/role';
import intention from 'consul-ui/forms/intention';

const builder = builderFactory();

const forms = {
  kv: kv,
  token: token,
  policy: policy,
  role: role,
  intention: intention,
};

export default class FormService extends Service {
  // a `get` method is added via the form initializer
  // see initializers/form.js

  // TODO: Temporarily add these here until something else needs
  // dynamic repos
  @service('repository/role') role;
  @service('repository/policy') policy;
  //
  forms = [];

  build(obj, name) {
    return builder(...arguments);
  }

  form(name) {
    let form = this.forms[name];
    if (typeof form === 'undefined') {
      form = this.forms[name] = forms[name](this);
      // only do special things for our new things for the moment
      if (name === 'role' || name === 'policy') {
        const repo = this[name];
        form.clear(function (obj) {
          return repo.create(obj);
        });
        form.submit(function (obj) {
          return repo.persist(obj);
        });
      }
    }
    return form;
  }
}
