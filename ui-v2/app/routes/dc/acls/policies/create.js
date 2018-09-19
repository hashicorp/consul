import Route from './edit';
import CreatingRoute from 'consul-ui/mixins/creating-route';

export default Route.extend(CreatingRoute, {
  templateName: 'dc/acls/policies/edit',
});
