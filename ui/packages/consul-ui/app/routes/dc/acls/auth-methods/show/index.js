import Route from 'consul-ui/routing/route';
import to from 'consul-ui/utils/routing/redirect-to';

export default Route.extend({
  redirect: to('auth-method'),
});
