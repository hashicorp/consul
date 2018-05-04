import Serializer from './application';
import { PRIMARY_KEY } from 'consul-ui/models/coordinate';

export default Serializer.extend({
  primaryKey: PRIMARY_KEY,
});
