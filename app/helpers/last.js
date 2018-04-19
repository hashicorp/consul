import { helper } from '@ember/component/helper';

export function last([obj = ''], hash) {
  switch (true) {
    case typeof obj === 'string':
      return obj.substr(-1);
  }
}

export default helper(last);
