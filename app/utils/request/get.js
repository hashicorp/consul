import formatURLFactory from 'consul-ui/utils/formatURL';
import request from 'consul-ui/utils/request/request';
const consulHost = '';
const formatURL = formatURLFactory();
export default function(url, dc) {
  return request(formatURL(consulHost + url, dc));
}
