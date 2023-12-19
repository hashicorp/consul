import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/auth-method';

export default class AuthMethodSerializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;
}
