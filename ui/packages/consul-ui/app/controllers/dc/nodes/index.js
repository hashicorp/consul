import PeeredResourceController from 'consul-ui/controllers/_peered-resource';
import { action } from '@ember/object';

export default class DcNodesController extends PeeredResourceController {
  @action
  dismissAgentlessNotice() {
    console.log('dismiss this here')
  }
}
