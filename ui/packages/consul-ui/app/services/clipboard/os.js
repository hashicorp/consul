import Service from '@ember/service';

import Clipboard from 'clipboard';

export default class OsService extends Service {
  execute() {
    return new Clipboard(...arguments);
  }
}
