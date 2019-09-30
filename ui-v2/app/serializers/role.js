import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/role';
import WithPolicies from 'consul-ui/mixins/policy/as-many';
export default Serializer.extend(WithPolicies, {
  primaryKey: PRIMARY_KEY,
  slugKey: SLUG_KEY,
});
