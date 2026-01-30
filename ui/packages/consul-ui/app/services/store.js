/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: BUSL-1.1
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
    const adapter = this.adapterFor(modelName);

    // Use peekRecord (public API) instead of _internalModelForId (removed in ember-data 4.x)
    const record = this.peekRecord(modelName, id);
    if (!record) {
      throw new Error(`Record not found: ${modelName}:${id}`);
    }

    // Try ember-data 3.x style first
    if (record._internalModel && typeof record._internalModel.createSnapshot === 'function') {
      return adapter.clone(
        this,
        { modelName: modelName },
        id,
        record._internalModel.createSnapshot()
      );
    }

    // ember-data 4.x+: create snapshot-like object
    // The serializer.serialize() needs eachAttribute, eachRelationship, attr, belongsTo, hasMany
    const modelClass = this.modelFor(modelName);
    const attrs = {};
    record.eachAttribute((name) => {
      attrs[name] = record[name];
    });

    const snapshot = {
      id: record.id,
      type: modelClass,
      modelName: modelName,
      record: record,

      // Return all attributes as object
      attributes: () => attrs,

      // Return single attribute value
      attr: (name) => attrs[name],

      // Iterate over attributes - required by JSONSerializer.serialize()
      eachAttribute: (callback) => {
        record.eachAttribute((name, meta) => {
          callback(name, meta);
        });
      },

      // Iterate over relationships - required by JSONSerializer.serialize()
      eachRelationship: (callback) => {
        record.eachRelationship((name, meta) => {
          callback(name, meta);
        });
      },

      // Handle belongsTo relationships
      belongsTo: (name) => {
        try {
          const rel = record.belongsTo(name);
          if (rel && typeof rel.value === 'function') {
            const value = rel.value();
            if (value) {
              return { id: value.id };
            }
          }
        } catch (e) {
          // relationship may not exist
        }
        return null;
      },

      // Handle hasMany relationships
      hasMany: (name) => {
        try {
          const rel = record.hasMany(name);
          if (rel && typeof rel.value === 'function') {
            const values = rel.value();
            if (values) {
              return values.map((v) => ({ id: v.id }));
            }
          }
        } catch (e) {
          // relationship may not exist
        }
        return null;
      },
    };

    const serializer = this.serializerFor(modelName);

    return adapter.clone(this, { modelName: modelName }, id, snapshot)
      .then((payload) => {
        const normalized = serializer.normalizeResponse(
          this, 
          modelClass, 
          payload, 
          id, 
          'findRecord' // Triggers standard findRecord normalization rules
        );
        return this.push(normalized);
      });
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
