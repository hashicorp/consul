import Route from 'consul-ui/routing/route';
import to from 'consul-ui/utils/routing/redirect-to';

export default class InstanceIndexRoute extends Route {
  redirect = to('healthchecks');
}
