import formatURLFactory from 'consul-ui/utils/formatURL';
import $ from 'jquery';
const consulHost = '';
const request = $.ajax;
const formatURL = formatURLFactory();
export default function(url, dc, data) {
  return request({
    url: formatURL(consulHost + url, dc),
    type: 'PUT',
    data: data,
  });
}
