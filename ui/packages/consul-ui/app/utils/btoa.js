import base64js from 'base64-js';
export default function(str) {
  // encode
  const bytes = new TextEncoder().encode(str);
  return base64js.fromByteArray(bytes);
}
