import { helper } from '@ember/component/helper';

export default helper(function(args, hash) {
  try {
    return JSON.stringify(...args);
  } catch(e) {
    return args[0].map(item => JSON.stringify(item, args[1], args[2]));
  }
});
