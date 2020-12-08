import Adapter from './application';

export default class DcAdapter extends Adapter {
  requestForQuery(request) {
    return request`
      GET /v1/catalog/datacenters
    `;
  }
}
