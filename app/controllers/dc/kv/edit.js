import Controller, { inject as controller } from '@ember/controller';
import { computed } from '@ember/object';

import { get as getter } from '@ember/object';

import get from 'consul-ui/utils/request/get';
import put from 'consul-ui/utils/request/put';
import del from 'consul-ui/utils/request/del';

export default Controller.extend({
  isLoading: false,
  isLockedOrLoading: computed.or('isLoading', 'isLocked'),
});
