/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { clickable } from 'ember-cli-page-object';
import { input, options, click, button } from 'consul-ui/tests/lib/page-object';
import powerSelect from 'consul-ui/components/power-select/pageobject';
import headersForm from 'consul-ui/components/consul/intention/permission/header/form/pageobject';
import headersList from 'consul-ui/components/consul/intention/permission/header/list/pageobject';

export default (scope = '.consul-intention-permission-form') => {
  return {
    scope: scope,
    resetScope: true, // where we use the form it is in a modal layer
    submit: {
      resetScope: true,
      scope: '.consul-intention-permission-modal [data-test-intention-permission-submit]',
      click: clickable(),
    },
    Action: {
      scope: '[data-property="action"]',
      ...options(['Allow', 'Deny']),
    },
    PathType: {
      scope: '[data-property="pathtype"]',
      ...powerSelect(['NoPath', 'PrefixedBy', 'Exact', 'RegEx']),
    },
    Path: {
      scope: '[data-property="path"] input',
      ...input(),
    },
    AllMethods: {
      scope: '[data-property="allmethods"]',
      ...click(),
    },
    Headers: {
      form: {
        ...headersForm(),
        submit: {
          resetScope: true,
          scope: '[data-test-add-header]',
          ...button(),
        },
      },
      list: {
        ...headersList(),
      },
    },
  };
};
