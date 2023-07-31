import Serializer from './application';
import { PRIMARY_KEY, SLUG_KEY } from 'consul-ui/models/proxy';

export default class ProxySerializer extends Serializer {
  primaryKey = PRIMARY_KEY;
  slugKey = SLUG_KEY;
  attrs = {
    NodeName: 'Node',
  };
}
