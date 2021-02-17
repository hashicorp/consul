import Adapter from './application';

export default class AuthMethodAdapter extends Adapter {
  requestForQuery(request, { dc, ns, index, id }) {
    return request`
      GET /v1/acl/auth-methods?${{ dc }}

      ${{
        ...this.formatNspace(ns),
        index,
      }}
    `;
  }

  requestForQueryRecord(request, { dc, ns, index, id }) {
    if (typeof id === 'undefined') {
      throw new Error('You must specify an id');
    }
    return request`
      GET /v1/acl/auth-method/${id}?${{ dc }}

      ${{
        ...this.formatNspace(ns),
        index,
      }}
    `;
  }
}
