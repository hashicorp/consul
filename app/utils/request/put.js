import formatURLFactory from 'consul-ui/utils/formatURL';
import request from 'consul-ui/utils/request/request';
const consulHost = '';
const formatURL = formatURLFactory();
export default function(url, dc, data) {
  return request({
    url: formatURL(consulHost + url, dc),
    type: 'PUT',
    data: data,
  });
}
