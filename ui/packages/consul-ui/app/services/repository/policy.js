/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
 */

import RepositoryService from 'consul-ui/services/repository';
import { get } from '@ember/object';
import { inject as service } from '@ember/service';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/policy';
import dataSource from 'consul-ui/decorators/data-source';

const MODEL_NAME = 'policy';

export default class PolicyService extends RepositoryService {
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

  @dataSource('/:partition/:ns/:dc/policies')
  async findAllByDatacenter() {
    return super.findAllByDatacenter(...arguments);
  }

  @dataSource('/:partition/:ns/:dc/policy/:id')
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

  persist(item) {
    // only if a policy doesn't have a template, save it
    // right now only ServiceIdentities have templates and
    // are not saveable themselves (but can be saved to a Role/Token)
    switch (get(item, 'template')) {
      case '':
        return item.save();
    }
    return Promise.resolve(item);
  }

  translate(item) {
    return this.store.translate('policy', get(item, 'Rules'));
  }
}
