/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

(services => services({

}))(
  (json, data = (typeof document !== 'undefined' ? document.currentScript.dataset : module.exports)) => {
    data[`services`] = JSON.stringify(json);
  }
);
