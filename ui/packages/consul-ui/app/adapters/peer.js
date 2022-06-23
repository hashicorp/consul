import JSONAPIAdapter from '@ember-data/adapter/json-api';

export default class PeerAdapter extends JSONAPIAdapter {
  namespace = 'v1';

  pathForType(_modelName) {
    return 'peerings';
  }
}
