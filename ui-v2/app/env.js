import config from './config/environment';
export default function(str) {
  const user = window.localStorage.getItem(str);
  return user !== null ? user : config[str];
}
