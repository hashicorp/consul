import base64js from 'base64-js';
export default function(str, encoding = 'utf-8') {
  // encode
  const bytes = new TextEncoder(encoding).encode(str);
  return base64js.fromByteArray(bytes);
}
