import Store from 'ember-data/store';
import { inject as service } from '@ember/service';

export default Store.extend({
  // TODO: This should eventually go on a static method
  // of the abstract Repository class
  http: service('repository/type/event-source'),
  dataSource: service('data-source/service'),
  client: service('client/http'),
  clear: function() {
    // Aborting the client will close all open http type sources
    this.client.abort();
    // once they are closed clear their caches
    this.http.resetCache();
    this.dataSource.resetCache();
    this.init();
  },
  //
  // TODO: These only exist for ACLs, should probably make sure they fail
  // nicely if you aren't on ACLs for good DX
  // cloning immediately refreshes the view
  clone: function(modelName, id) {
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
  },
  self: function(modelName, token) {
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
      .then(payload => serializer.normalizeResponse(this, modelClass, payload, token, 'self'));
  },
  //
  // TODO: This one is only for nodes, should fail nicely if you call it
  // for anything other than nodes for good DX
  queryLeader: function(modelName, query) {
    // TODO: no normalization, type it properly for the moment
    const adapter = this.adapterFor(modelName);
    return adapter.queryLeader(this, { modelName: modelName }, null, query);
  },
  // TODO: This one is only for nspaces and OIDC, should fail nicely if you call it
  // for anything other than nspaces/OIDC for good DX
  authorize: function(modelName, query = {}) {
    const adapter = this.adapterFor(modelName);
    const serializer = this.serializerFor(modelName);
    const modelClass = { modelName: modelName };
    return adapter
      .authorize(this, modelClass, null, query)
      .then(payload =>
        serializer.normalizeResponse(this, modelClass, payload, undefined, 'authorize')
      );
  },
  logout: function(modelName, query = {}) {
    const adapter = this.adapterFor(modelName);
    const modelClass = { modelName: modelName };
    return adapter.logout(this, modelClass, query.id, query);
  },
});
