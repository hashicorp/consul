import { REQUEST_CREATE, REQUEST_UPDATE } from 'consul-ui/adapters/application';

import Mixin from '@ember/object/mixin';

import minimizeModel from 'consul-ui/utils/minimizeModel';

export default Mixin.create({
  handleSingleResponse: function(url, response, primary, slug) {
    ['Roles'].forEach(function(prop) {
      if (typeof response[prop] === 'undefined' || response[prop] === null) {
        response[prop] = [];
      }
    });
    return this._super(url, response, primary, slug);
  },
  dataForRequest: function(params) {
    const name = params.type.modelName;
    const data = this._super(...arguments);
    switch (params.requestType) {
      case REQUEST_UPDATE:
      // falls through
      case REQUEST_CREATE:
        data[name].Roles = minimizeModel(data[name].Roles);
        break;
    }
    return data;
  },
});
