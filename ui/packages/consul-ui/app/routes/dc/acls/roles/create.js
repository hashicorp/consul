import Route from './edit';
import CreatingRoute from 'consul-ui/mixins/creating-route';

export default class CreateRoute extends Route.extend(CreatingRoute) {
  templateName = 'dc/acls/roles/edit';
}
