/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

import { inject as service } from '@ember/service';
import Store from '@ember-data/store';

export default class StoreService extends Store {
  @service('data-source/service') dataSource;

  @service('client/http') client;

  invalidate(status = 401) {
    // Aborting the client will close all open http type sources
    this.client.abort(401);
    // once they are closed clear their caches
    this.dataSource.resetCache();
    this.init();
  }

  clear() {
    this.invalidate(0);
  }

  //
  // TODO: These only exist for ACLs, should probably make sure they fail
  // nicely if you aren't on ACLs for good DX
  // cloning immediately refreshes the view
  clone(modelName, id) {
    // TODO: no normalization, type it properly for the moment
    const adapter = this.adapterFor(modelName);
    // passing empty options gives me a snapshot - ?
    const options = {};
    // _internalModel starts with _ but isn't marked as private ?
    return adapter.clone(
      this,
      { modelName: modelName },
      id,
      this._internalModelForId(modelName, id).createSnapshot(options)
    );
    // TODO: See https://github.com/emberjs/data/blob/7b8019818526a17ee72747bd3c0041354e58371a/addon/-private/system/promise-proxies.js#L68
  }

  self(modelName, token) {
    // TODO: no normalization, type it properly for the moment
    const adapter = this.adapterFor(modelName);
    const serializer = this.serializerFor(modelName);
    const modelClass = { modelName: modelName };
    // self is the only custom store method that goes through the serializer for the moment
    // this means it will have its meta data set correctly
    // if other methods need meta adding, then this should be carried over to
    // other methods. Ideally this would have been done from the outset
    // TODO: Carry this over to the other methods ^
    return adapter
      .self(this, modelClass, token.secret, token)
      .then((payload) => serializer.normalizeResponse(this, modelClass, payload, token, 'self'));
  }

  //
  // TODO: This one is only for nodes, should fail nicely if you call it
  // for anything other than nodes for good DX
  queryLeader(modelName, query) {
    const adapter = this.adapterFor(modelName);
    const serializer = this.serializerFor(modelName);
    const modelClass = { modelName: modelName };
    return adapter.queryLeader(this, modelClass, null, query).then((payload) => {
      payload.meta = serializer.normalizeMeta(this, modelClass, payload, null, 'leader');
      return payload;
    });
  }

  // TODO: This one is only for permissions and OIDC, should fail nicely if you call it
  // for anything other than permissions/OIDC for good DX
  authorize(modelName, query = {}) {
    const adapter = this.adapterFor(modelName);
    const serializer = this.serializerFor(modelName);
    const modelClass = { modelName: modelName };
    return adapter
      .authorize(this, modelClass, null, query)
      .then((payload) =>
        serializer.normalizeResponse(this, modelClass, payload, undefined, 'authorize')
      );
  }

  logout(modelName, query = {}) {
    const adapter = this.adapterFor(modelName);
    const modelClass = { modelName: modelName };
    return adapter.logout(this, modelClass, query.id, query);
  }
}
