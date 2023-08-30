/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import RepositoryService from 'consul-ui/services/repository';
import { inject as service } from '@ember/service';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/role';
import dataSource from 'consul-ui/decorators/data-source';

const MODEL_NAME = 'role';

export default class RoleService extends RepositoryService {
  @service('form') form;
  getModelName() {
    return MODEL_NAME;
  }

  getPrimaryKey() {
    return PRIMARY_KEY;
  }

  getSlugKey() {
    return SLUG_KEY;
  }

  @dataSource('/:partition/:ns/:dc/roles')
  async findAll() {
    return super.findAll(...arguments);
  }

  @dataSource('/:partition/:ns/:dc/role/:id')
  async findBySlug(params) {
    let item;
    if (params.id === '') {
      item = await this.create({
        Datacenter: params.dc,
        Partition: params.partition,
        Namespace: params.ns,
      });
    } else {
      item = await super.findBySlug(...arguments);
    }
    return this.form.form(this.getModelName()).setData(item).getData();
  }
}
