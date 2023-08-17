/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

export default function (scenario, create, set, win = window, doc = document) {
  scenario
    .given(['an external edit results in $number $model model[s]?'], function (number, model) {
      return create(number, model);
    })
    .given(['$number $model model[s]?'], function (number, model) {
      return create(number, model);
    })
    .given(['$number $model model[s]? with the value "$value"'], function (number, model, value) {
      if (model === 'dc') {
        doc.cookie = `CONSUL_DATACENTER_LOCAL=${value}`;
      }
      return create(number, model, value);
    })
    .given(
      ['$number $model model[s]? from yaml\n$yaml', '$number $model model[s]? from json\n$json'],
      function (number, model, data) {
        if (model === 'dc') {
          doc.cookie = `CONSUL_DATACENTER_LOCAL=${data[0]}`;
        }
        return create(number, model, data);
      }
    )
    .given(['settings from yaml\n$yaml'], function (data) {
      return Object.keys(data).forEach(function (key) {
        win.localStorage[key] = JSON.stringify(data[key]);
      });
    })
    .given(['ui_config from yaml\n$yaml'], function (data) {
      // this one doesn't interact with the api therefore you don't need to use
      // setCookie/set. Ideally setCookie should probably use doc.cookie also so
      // there is no difference between these
      doc.cookie = `CONSUL_UI_CONFIG=${JSON.stringify(data)}`;
    })
    .given(['the local datacenter is "$value"'], function (value) {
      doc.cookie = `CONSUL_DATACENTER_LOCAL=${value}`;
    })
    .given(['ACLs are disabled'], function () {
      doc.cookie = `CONSUL_ACLS_ENABLE=0`;
    })
    .given(['SSO is enabled'], function () {
      doc.cookie = `CONSUL_SSO_ENABLE=1`;
      set('CONSUL_SSO_ENABLE', 1);
    })
    .given(['partitions are enabled'], function () {
      doc.cookie = `CONSUL_PARTITIONS_ENABLE=1`;
      set('CONSUL_PARTITIONS_ENABLE', 1);
    })
    .given(['the default ACL policy is "$policy"'], function (policy) {
      set('CONSUL_ACL_POLICY', policy);
    })
    .given(['a "$value" metrics provider'], function (value) {
      doc.cookie = `CONSUL_METRICS_PROXY_ENABLE=1`;
      doc.cookie = `CONSUL_METRICS_PROVIDER=${value}`;
    })
    .given(['permissions from yaml\n$yaml'], function (data) {
      Object.entries(data).forEach(([key, value]) => {
        const resource = `CONSUL_RESOURCE_${key.toUpperCase()}`;
        Object.entries(value).forEach(([key, value]) => {
          set(`${resource}_${key.toUpperCase()}`, value);
        });
      });
    });
}
