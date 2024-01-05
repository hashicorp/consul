/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import Service, { inject as service } from '@ember/service';
import { get } from '@ember/object';

import dataSource from 'consul-ui/decorators/data-source';

export default class UiConfigService extends Service {
  @service('env') env;

  @dataSource('/:partition/:nspace/:dc/ui-config/:path')
  async findByPath(params) {
    return get(this.get(), params.path);
  }

  @dataSource('/:partition/:nspace/:dc/notfound/:path')
  async parsePath(params) {
    return params.path.split('/').reduce((prev, item, i) => {
      switch (true) {
        case item.startsWith('~'):
          prev.nspace = item.substr(1);
          break;
        case item.startsWith('_'):
          prev.partition = item.substr(1);
          break;
        case typeof prev.dc === 'undefined':
          prev.dc = item;
          break;
      }
      return prev;
    }, {});
  }

  @dataSource('/:partition/:nspace/:dc/ui-config')
  async get() {
    return this.env.var('CONSUL_UI_CONFIG');
  }

  // @deprecated
  getSync() {
    return this.env.var('CONSUL_UI_CONFIG');
  }
}
