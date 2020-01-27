import Store from 'ember-data/store';

export default Store.extend({
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
    return adapter.self(this, { modelName: modelName }, token.secret, token);
  },
  //
  // TODO: This one is only for nodes, should fail nicely if you call it
  // for anything other than nodes for good DX
  queryLeader: function(modelName, query) {
    // TODO: no normalization, type it properly for the moment
    const adapter = this.adapterFor(modelName);
    return adapter.queryLeader(this, { modelName: modelName }, null, query);
  },
  // TODO: This one is only for ACL, should fail nicely if you call it
  // for anything other than ACLs for good DX
  authorize: function(modelName, query = {}) {
    // TODO: no normalization, type it properly for the moment
    return this.adapterFor(modelName).authorize(this, { modelName: modelName }, null, query);
  },
});
