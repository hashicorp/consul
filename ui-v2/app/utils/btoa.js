import TextEncoderLite from 'npm:text-encoder-lite';
import base64js from 'npm:base64-js';
export default function(str, encoding = 'utf-8') {
  // encode
  const bytes = new (TextEncoder || TextEncoderLite)(encoding).encode(str);
  return base64js.fromByteArray(bytes);
}
