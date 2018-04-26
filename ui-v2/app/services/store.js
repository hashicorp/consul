import Service from '@ember/service';
import Store from 'ember-data/store';
import { get } from '@ember/object';

export default Store.extend({
  // luckily I do another query straight after this, so the cloned
  // item will end up being loaded in via the api straight after this
  // as long as this returns a promise on success we should be good
  clone: function(modelName, id) {
    // no normalization, type it properly
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
    // https://github.com/emberjs/data/blob/7b8019818526a17ee72747bd3c0041354e58371a/addon/-private/system/promise-proxies.js#L68
    // .then(
    //   function(internalModel) {
    //     return internalModel;
    //     return {promise: Promise.resolve(internalModel)}
    //   }
    // );
  },
});
