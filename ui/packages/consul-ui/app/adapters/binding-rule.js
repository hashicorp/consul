import Adapter from './application';

export default class BindingRuleAdapter extends Adapter {
  requestForQuery(request, { dc, ns, partition, authmethod, index }) {
    return request`
      GET /v1/acl/binding-rules?${{ dc, authmethod }}

      ${{
        ns,
        partition,
        index,
      }}
    `;
  }
}
