import Route from '@ember/routing/route';
import to from 'consul-ui/utils/routing/redirect-to';

export default Route.extend({
  redirect: to('instances'),
});
