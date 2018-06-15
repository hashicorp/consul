import getAPI from '@hashicorp/ember-cli-api-double';
import setCookies from 'consul-ui/tests/helpers/set-cookies';
import typeToURL from 'consul-ui/tests/helpers/type-to-url';
export default getAPI('/consul-api-double', setCookies, typeToURL);
