import Route from './edit';
import CreatingRoute from 'consul-ui/mixins/creating-route';

export default Route.extend(CreatingRoute, {
  templateName: 'dc/nspaces/edit',
  beforeModel: function() {
    // we need to skip CreatingRoute.beforeModel here
    // TODO(octane): ideally we'd like to call Route.beforeModel
    // but its not clear how to do that with old ember
    // maybe it will be more normal with modern ember
    // up until now we haven't been calling super here anyway
    // so this is probably ok until we can skip a parent super
  },
});
