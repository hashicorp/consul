(services => services({
  "component:consul/peer/selector": {
    "class": "consul-ui/components/consul/peer/selector"
  }
}))(
  (json, data = (typeof document !== 'undefined' ? document.currentScript.dataset : module.exports)) => {
    data[`services`] = JSON.stringify(json);
  }
);
