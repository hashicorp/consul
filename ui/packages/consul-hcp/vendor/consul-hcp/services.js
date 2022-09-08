(services => services({
  'component:consul/hcp/home': {
    class: 'consul-ui/components/consul/hcp/home',
  },
}))(
  (json, data = (typeof document !== 'undefined' ? document.currentScript.dataset : module.exports)) => {
    data[`services`] = JSON.stringify(json);
  }
);
