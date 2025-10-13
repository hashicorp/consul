/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

'use strict';

module.exports = {
  extends: 'recommended',
  rules: {
    'no-partial': false,
    'table-groups': false,

    'no-invalid-interactive': false,
    'simple-unless': false,

    'self-closing-void-elements': false,
    'no-unnecessary-concat': false,
    'no-quoteless-attributes': false,
    'no-nested-interactive': false,

    'block-indentation': false,
    quotes: false,

    'no-inline-styles': false,
    'no-triple-curlies': false,
    'no-unused-block-params': false,
    'style-concatenation': false,
    'link-rel-noopener': false,

    'no-implicit-this': {
  allow: [
    // Template variables commonly used in partials
    'item',
    'item.isPristine',
    'item.isInvalid', 
    'item.AccessorID',
    'item.Description',
    'item.Roles',
    'item.Policies',
    'item.Rules',
    'item.Type',
    'item.ID',
    'item.Name',
    'item.error.Name',
    'item.error.Type',
    'items',
    'items.length',
    'create',
    'token',
    // Route params passed as template variables
    'dc',
    'partition', 
    'nspace',
    'id',
    // Form field variables
    'Local',
    'name',
    'form'
  ]
},
// 'no-implicit-this': 'error',
    'no-curly-component-invocation': false,
    'no-action': false,
    'no-negated-condition': false,
    'no-invalid-role': false,

    'no-unnecessary-component-helper': false,
    'link-href-attributes': false,
    // we need to be able to say tabindex={{@tabindex}}
    'no-positive-tabindex': false,

    'no-bare-strings': false,
  },
};
