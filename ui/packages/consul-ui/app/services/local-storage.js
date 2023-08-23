import Service from '@ember/service';
import { getOwner } from '@ember/application';
import ENV from 'consul-ui/config/environment';

export function storageFor(key) {
  return function () {
    return {
      get() {
        const owner = getOwner(this);

        const localStorageService = owner.lookup('service:localStorage');

        return localStorageService.getBucket(key);
      },
    };
  };
}

/**
 * An in-memory stub of window.localStorage. Ideally this would
 * implement the [Storage](https://developer.mozilla.org/en-US/docs/Web/API/Storage)-interface that localStorage implements
 * as well.
 *
 * We use this implementation during testing to not pollute `window.localStorage`
 */
class MemoryStorage {
  constructor() {
    this.data = new Map();
  }

  getItem(key) {
    return this.data.get(key);
  }

  setItem(key, value) {
    return this.data.set(key, value.toString());
  }

  /**
   * A function to seed data into MemoryStorage. This expects an object to be
   * passed. The passed values will be persisted as a string - i.e. the values
   * passed will call their `toString()`-method before writing to storage. You need
   * to  take this into account when you want to persist complex values, like arrays
   * or objects:
   *
   * Example:
   *
   * ```js
   * const storage = new MemoryStorage();
   * storage.seed({ notices: ['notice-a', 'notice-b']});
   *
   * storage.getItem('notices') // => 'notice-a,notice-b'
   *
   * // won't work
   * storage.seed({
   *   user: { name: 'Tomster' }
   * })
   *
   * storage.getItem('user') // => '[object Object]'
   *
   * // this works
   * storage.seed({
   * .  user: JSON.stringify({name: 'Tomster'})
   * })
   *
   * storage.getItem('user') // => '{ "name": "Tomster" }'
   * ```
   * @param {object} data - the data to seed
   */
  seed(data) {
    const newData = new Map();

    const keys = Object.keys(data);

    keys.forEach((key) => {
      newData.set(key, data[key].toString());
    });

    this.data = newData;
  }
}

/**
 * There might be better ways to do this but this is good enough for now.
 * During testing we want to use MemoryStorage not window.localStorage.
 */
function initStorage() {
  if (ENV.environment === 'test') {
    return new MemoryStorage();
  } else {
    return window.localStorage;
  }
}

/**
 * A service that wraps access to local-storage. We wrap
 * local-storage to not pollute local-storage during testing.
 */
export default class LocalStorageService extends Service {
  constructor() {
    super(...arguments);

    this.storage = initStorage();
    this.buckets = new Map();
  }

  getBucket(key) {
    const bucket = this.buckets.get(key);

    if (bucket) {
      return bucket;
    } else {
      return this._setupBucket(key);
    }
  }

  _setupBucket(key) {
    const owner = getOwner(this);
    const Klass = owner.factoryFor(`storage:${key}`).class;
    const storage = new Klass(key, this.storage);

    this.buckets.set(key, storage);

    return storage;
  }
}
