import Adapter from './application';

export default class BindingRuleAdapter extends Adapter {
  requestForQuery(request, { dc, ns, authmethod, index, id }) {
    return request`
      GET /v1/acl/binding-rules?${{ dc, authmethod }}

      ${{
        ...this.formatNspace(ns),
        index,
      }}
    `;
  }
}
