/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { inject as service } from '@ember/service';
import { runInDebug } from '@ember/debug';
import RepositoryService, { softDelete } from 'consul-ui/services/repository';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/partition';
import dataSource from 'consul-ui/decorators/data-source';

import { defaultChangeset as changeset } from 'consul-ui/utils/form/builder';

const findActive = function (items, item) {
  let found = items.find(function (i) {
    return i.Name === item.Name;
  });
  if (typeof found === 'undefined') {
    runInDebug((_) =>
      console.info(`${item.Name} not found in [${items.map((item) => item.Name).join(', ')}]`)
    );
    // if we can't find the nspace that was specified try default
    found = items.find(function (item) {
      return item.Name === 'default';
    });
    // if there is no default just choose the first
    if (typeof found === 'undefined') {
      found = items[0];
    }
  }
  return found;
};

const MODEL_NAME = 'partition';
export default class PartitionRepository extends RepositoryService {
  @service('settings') settings;
  @service('form') form;
  @service('repository/permission') permissions;

  getModelName() {
    return MODEL_NAME;
  }

  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  getSlugKey() {
    return SLUG_KEY;
  }

  @dataSource('/:partition/:ns/:dc/partitions')
  async findAll() {
    if (!this.permissions.can('use partitions')) {
      return [];
    }
    return super.findAll(...arguments).catch(() => []);
  }

  @dataSource('/:partition/:ns/:dc/partition/:id')
  async findBySlug(params) {
    let item;
    if (params.id === '') {
      item = await this.create({
        Datacenter: params.dc,
        Partition: '',
      });
    } else {
      item = await super.findBySlug(...arguments);
    }
    return changeset(item);
  }

  remove(item) {
    return softDelete(this, item);
  }

  async getActive(currentName = '') {
    const type = 'partition';
    const items = this.store.peekAll(type).toArray();
    if (currentName.length === 0) {
      const token = await this.settings.findBySlug('token');
      currentName = token['Partition'] || 'default';
    }
    // if there is only 1 item then use that, otherwise find the
    // item object that corresponds to the active one
    return items.length === 1 ? items[0] : findActive(items, { Name: currentName });
  }
}
