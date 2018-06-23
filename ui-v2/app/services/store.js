import Store from 'ember-data/store';

export default Store.extend({
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
});
