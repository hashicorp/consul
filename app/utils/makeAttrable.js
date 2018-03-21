// Used to make an pojo 'attr-able'
// i.e. you can call pojo.attr('key') on it
export default function(obj) {
  return {
    attr: function(prop) {
      return obj[prop];
    },
  };
}
