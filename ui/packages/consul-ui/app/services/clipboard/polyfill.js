/* global ClipboardJS*/
import Service from '@ember/service';

export default class PolyfillService extends Service {
  execute() {
    // Access the ClipboardJS lib see vendor/init.js for polyfill loading
    return new ClipboardJS(...arguments);
  }
}
