/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

export default function (scenario, respondWith, set, oidc) {
  // respondWith should set the url to return a certain response shape
  scenario
    .given(['the url "$endpoint" responds with a $status status'], function (url, status) {
      respondWith(url, {
        status: parseInt(status),
      });
    })
    .given(['the url "$endpoint" responds with from yaml\n$yaml'], function (url, data) {
      if (typeof data.body !== 'string') {
        data.body = JSON.stringify(data.body);
      }
      respondWith(url, data);
    })
    .given(['the "$provider" oidcProvider responds with from yaml\n$yaml'], function (name, data) {
      oidc(name, data);
    })
    .given('a network latency of $number', function (number) {
      set('CONSUL_LATENCY', number);
    })
    .given('an API prefix of "$prefix"', function (prefix) {
      set('CONSUL_API_PREFIX', prefix);
    });
}
