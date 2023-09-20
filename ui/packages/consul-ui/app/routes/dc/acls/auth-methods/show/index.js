import Route from 'consul-ui/routing/route';
import to from 'consul-ui/utils/routing/redirect-to';

export default class AuthMethodShowIndexRoute extends Route {
  redirect = to('auth-method');
}
