import Serializer from './application';
import { PRIMARY_KEY } from 'consul-ui/models/role';
export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
});
