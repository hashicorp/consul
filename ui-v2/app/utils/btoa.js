import TextEncoding from 'npm:text-encoding';
import base64js from 'npm:base64-js';
export default function(str, encoding = 'utf-8') {
  // encode
  const bytes = new ('TextEncoder' in window ? TextEncoder : TextEncoding.TextEncoder)(encoding).encode(str);
  return base64js.fromByteArray(bytes);
}
