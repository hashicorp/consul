import TextEncoding from 'npm:text-encoding';
import base64js from 'npm:base64-js';
export default function(str, encoding = 'utf-8') {
  // decode
  const bytes = base64js.toByteArray(str);
  return new ('TextDecoder' in window ? TextDecoder : TextEncoding.TextDecoder)(encoding).decode(
    bytes
  );
}
