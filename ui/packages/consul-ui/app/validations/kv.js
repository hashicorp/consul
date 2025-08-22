/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import { validatePresence, validateLength, validateFormat } from 'ember-changeset-validations/validators';

export default {
  Key: [
    validatePresence(true), 
    validateLength({ min: 1 }),
    validateFormat({ 
      regex: /^[^/].*$/,
      message: 'Key must not begin with a forward slash (/)' 
    }),
    validateFormat({ 
      regex: /^(?!.*\.\.[/\\]).*$/,
      message: 'Key contains invalid path traversal sequence (../)' 
    }),
    validateFormat({ 
      regex: /^(?!.*%2e%2e[%/\\]).*$/i,
      message: 'Key contains encoded path traversal sequence' 
    }),
    validateFormat({
      regex: /^(?!.*\.(js|css|html?|php|asp|jsp|exe|dll|pdf|doc|zip|jpg|png|gif|svg|mp[34]|woff2?|ttf|eot)(\?|$))/i,
      message: 'Key contains file extension that may be cached by proxies and could lead to security issues'
    })
  ],
};
