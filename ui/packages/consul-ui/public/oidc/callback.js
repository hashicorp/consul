/**
 * Copyright IBM Corp. 2024, 2026
 * SPDX-License-Identifier: BUSL-1.1
 */

var CURRENT_REQUEST_KEY = '__torii_request';
var pendingRequestKey = window.localStorage.getItem(CURRENT_REQUEST_KEY);

if (pendingRequestKey) {
  window.localStorage.removeItem(CURRENT_REQUEST_KEY);
  var url = window.location.toString();
  window.localStorage.setItem(pendingRequestKey, url);
}

window.close();
