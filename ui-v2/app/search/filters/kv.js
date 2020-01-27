import { get } from '@ember/object';
import rightTrim from 'consul-ui/utils/right-trim';
export default function(filterable) {
  return filterable(function(item, { s = '' }) {
    const key = rightTrim(get(item, 'Key'), '/')
      .split('/')
      .pop();
    return key.toLowerCase().indexOf(s.toLowerCase()) !== -1;
  });
}
