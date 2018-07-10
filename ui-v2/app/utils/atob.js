import TextEncoderLite from 'npm:text-encoder-lite';
import base64js from 'npm:base64-js';
export default function(str, encoding = 'utf-8') {
  // str = String(str).trim();
  //decode
  const bytes = base64js.toByteArray(str);
  return new (TextDecoder || TextEncoderLite)(encoding).decode(bytes);
}
