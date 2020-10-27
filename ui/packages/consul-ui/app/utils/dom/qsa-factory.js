export default function(doc = document) {
  return function(sel, context = doc) {
    return context.querySelectorAll(sel);
  };
}
