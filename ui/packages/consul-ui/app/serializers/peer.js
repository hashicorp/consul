import JSONAPISerializer from '@ember-data/serializer/json-api';

export default class PeerSerializer extends JSONAPISerializer {
  keyForAttribute(key) {
    return key.capitalize();
  }

  normalizeFindAllResponse(store, primaryModelClass, payload, id, requestType) {
    const data = payload.map(peering => {
      return {
        type: 'peer',
        id: peering.ID,
        attributes: {
          ...peering,
        },
      };
    });

    return super.normalizeFindAllResponse(store, primaryModelClass, { data }, id, requestType);
  }
}
