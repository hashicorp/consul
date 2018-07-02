import getAPI from '@hashicorp/ember-cli-api-double';
import setCookies from 'consul-ui/tests/helpers/set-cookies';
import typeToURL from 'consul-ui/tests/helpers/type-to-url';
import config from 'consul-ui/config/environment';
const apiConfig = config['ember-cli-api-double'];
let path = '/consul-api-double';
let reader;
if (apiConfig) {
  const temp = apiConfig.endpoints[0].split('/');
  reader = apiConfig.reader;
  temp.pop();
  path = temp.join('/');
}
export default getAPI(path, setCookies, typeToURL, reader);
