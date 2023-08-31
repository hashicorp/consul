/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import RepositoryService from 'consul-ui/services/repository';
import isFolder from 'consul-ui/utils/isFolder';
import { get } from '@ember/object';
import { PRIMARY_KEY } from 'consul-ui/models/kv';
// import { ACCESS_LIST } from 'consul-ui/abilities/base';
import dataSource from 'consul-ui/decorators/data-source';

const modelName = 'kv';
export default class KvService extends RepositoryService {
  getModelName() {
    return modelName;
  }

  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  shouldReconcile(item, params) {
    return super.shouldReconcile(...arguments) && item.Key.startsWith(params.id);
  }

  // this one gives you the full object so key,values and meta
  @dataSource('/:partition/:ns/:dc/kv/:id')
  async findBySlug(params, configuration = {}) {
    let item;
    if (isFolder(params.id)) {
      // we only use findBySlug for a folder when we are looking to create a
      // parent for a key for retrieving something Model shaped. Therefore we
      // only use existing records or a fake record with the correct Key,
      // which means we don't need to inspect permissions as its an already
      // existing KV or a fake one

      // TODO: This very much shouldn't be here,
      // needs to eventually use ember-datas generateId thing
      // in the meantime at least our fingerprinter
      const uid = JSON.stringify([params.partition, params.ns, params.dc, params.id]);
      item = this.store.peekRecord(this.getModelName(), uid);
      if (!item) {
        item = await this.create({
          Key: params.id,
          Datacenter: params.dc,
          Namespace: params.ns,
          Partition: params.partition,
        });
      }
    } else {
      if (params.id === '') {
        item = await this.create({
          Datacenter: params.dc,
          Namespace: params.ns,
          Partition: params.partition,
        });
      } else {
        item = await super.findBySlug(...arguments);
      }
    }
    // TODO: Whilst KV is using DataForm and DataForm does the model >
    // changeset conversion a model > changeset conversion is not needed here
    // until we move KV to just use DataWriter like the other new stuff
    return item;
  }

  // this one only gives you keys
  // https://www.consul.io/api/kv.html
  @dataSource('/:partition/:ns/:dc/kvs/:id')
  async findAllBySlug(params, configuration = {}) {
    params.separator = '/';
    if (params.id === '/') {
      params.id = '';
    }

    /**/
    // Temporarily revert to pre-1.10 UI functionality by not pre-checking backend
    // permissions.
    // This temporary measure should be removed again once https://github.com/hashicorp/consul/issues/11098
    // has been resolved

    // return this.authorizeBySlug(
    //   async () => {
    let items = await this.findAll(...arguments);
    const meta = items.meta;
    items = items.filter((item) => params.id !== get(item, 'Key'));
    items.meta = meta;
    return items;
    // },
    // ACCESS_LIST,
    // params
    // );
    /**/
  }
}
