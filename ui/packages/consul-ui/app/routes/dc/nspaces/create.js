import Route from './edit';
import CreatingRoute from 'consul-ui/mixins/creating-route';

export default class CreateRoute extends Route.extend(CreatingRoute) {
  templateName = 'dc/nspaces/edit';

  async beforeModel() {
    // TODO: Update nspace CRUD to use Data components
    // we need to skip CreatingRoute.beforeModel here
    // but still call Route.beforeModel
    return Route.prototype.beforeModel.apply(this, arguments);
  }
}
