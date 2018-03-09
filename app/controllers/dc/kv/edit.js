import Controller from '@ember/controller';
import { computed } from '@ember/object';

export default Controller.extend({
  isLockedOrLoading: computed.or('isLoading', 'isLocked'),
});
