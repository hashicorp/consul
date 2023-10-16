(function(n){"use strict"
function e(n,e,r){return e<=n&&n<=r}"undefined"!=typeof module&&module.exports&&!n["encoding-indexes"]&&(n["encoding-indexes"]=require("./encoding-indexes-50f27403be5972eae4831f5b69db1f80.js")["encoding-indexes"])
var r=Math.floor
function i(n){if(void 0===n)return{}
if(n===Object(n))return n
throw TypeError("Could not convert argument to dictionary")}function t(n){return 0<=n&&n<=127}var o=t,s=-1
function a(n){this.tokens=[].slice.call(n),this.tokens.reverse()}a.prototype={endOfStream:function(){return!this.tokens.length},read:function(){return this.tokens.length?this.tokens.pop():s},prepend:function(n){if(Array.isArray(n))for(var e=n;e.length;)this.tokens.push(e.pop())
else this.tokens.push(n)},push:function(n){if(Array.isArray(n))for(var e=n;e.length;)this.tokens.unshift(e.shift())
else this.tokens.unshift(n)}}
var u=-1
function l(n,e){if(n)throw TypeError("Decoder error")
return e||65533}function f(n){throw TypeError("The code point "+n+" could not be encoded.")}function c(n){return n=String(n).trim().toLowerCase(),Object.prototype.hasOwnProperty.call(h,n)?h[n]:null}var d=[{encodings:[{labels:["unicode-1-1-utf-8","utf-8","utf8"],name:"UTF-8"}],heading:"The Encoding"},{encodings:[{labels:["866","cp866","csibm866","ibm866"],name:"IBM866"},{labels:["csisolatin2","iso-8859-2","iso-ir-101","iso8859-2","iso88592","iso_8859-2","iso_8859-2:1987","l2","latin2"],name:"ISO-8859-2"},{labels:["csisolatin3","iso-8859-3","iso-ir-109","iso8859-3","iso88593","iso_8859-3","iso_8859-3:1988","l3","latin3"],name:"ISO-8859-3"},{labels:["csisolatin4","iso-8859-4","iso-ir-110","iso8859-4","iso88594","iso_8859-4","iso_8859-4:1988","l4","latin4"],name:"ISO-8859-4"},{labels:["csisolatincyrillic","cyrillic","iso-8859-5","iso-ir-144","iso8859-5","iso88595","iso_8859-5","iso_8859-5:1988"],name:"ISO-8859-5"},{labels:["arabic","asmo-708","csiso88596e","csiso88596i","csisolatinarabic","ecma-114","iso-8859-6","iso-8859-6-e","iso-8859-6-i","iso-ir-127","iso8859-6","iso88596","iso_8859-6","iso_8859-6:1987"],name:"ISO-8859-6"},{labels:["csisolatingreek","ecma-118","elot_928","greek","greek8","iso-8859-7","iso-ir-126","iso8859-7","iso88597","iso_8859-7","iso_8859-7:1987","sun_eu_greek"],name:"ISO-8859-7"},{labels:["csiso88598e","csisolatinhebrew","hebrew","iso-8859-8","iso-8859-8-e","iso-ir-138","iso8859-8","iso88598","iso_8859-8","iso_8859-8:1988","visual"],name:"ISO-8859-8"},{labels:["csiso88598i","iso-8859-8-i","logical"],name:"ISO-8859-8-I"},{labels:["csisolatin6","iso-8859-10","iso-ir-157","iso8859-10","iso885910","l6","latin6"],name:"ISO-8859-10"},{labels:["iso-8859-13","iso8859-13","iso885913"],name:"ISO-8859-13"},{labels:["iso-8859-14","iso8859-14","iso885914"],name:"ISO-8859-14"},{labels:["csisolatin9","iso-8859-15","iso8859-15","iso885915","iso_8859-15","l9"],name:"ISO-8859-15"},{labels:["iso-8859-16"],name:"ISO-8859-16"},{labels:["cskoi8r","koi","koi8","koi8-r","koi8_r"],name:"KOI8-R"},{labels:["koi8-ru","koi8-u"],name:"KOI8-U"},{labels:["csmacintosh","mac","macintosh","x-mac-roman"],name:"macintosh"},{labels:["dos-874","iso-8859-11","iso8859-11","iso885911","tis-620","windows-874"],name:"windows-874"},{labels:["cp1250","windows-1250","x-cp1250"],name:"windows-1250"},{labels:["cp1251","windows-1251","x-cp1251"],name:"windows-1251"},{labels:["ansi_x3.4-1968","ascii","cp1252","cp819","csisolatin1","ibm819","iso-8859-1","iso-ir-100","iso8859-1","iso88591","iso_8859-1","iso_8859-1:1987","l1","latin1","us-ascii","windows-1252","x-cp1252"],name:"windows-1252"},{labels:["cp1253","windows-1253","x-cp1253"],name:"windows-1253"},{labels:["cp1254","csisolatin5","iso-8859-9","iso-ir-148","iso8859-9","iso88599","iso_8859-9","iso_8859-9:1989","l5","latin5","windows-1254","x-cp1254"],name:"windows-1254"},{labels:["cp1255","windows-1255","x-cp1255"],name:"windows-1255"},{labels:["cp1256","windows-1256","x-cp1256"],name:"windows-1256"},{labels:["cp1257","windows-1257","x-cp1257"],name:"windows-1257"},{labels:["cp1258","windows-1258","x-cp1258"],name:"windows-1258"},{labels:["x-mac-cyrillic","x-mac-ukrainian"],name:"x-mac-cyrillic"}],heading:"Legacy single-byte encodings"},{encodings:[{labels:["chinese","csgb2312","csiso58gb231280","gb2312","gb_2312","gb_2312-80","gbk","iso-ir-58","x-gbk"],name:"GBK"},{labels:["gb18030"],name:"gb18030"}],heading:"Legacy multi-byte Chinese (simplified) encodings"},{encodings:[{labels:["big5","big5-hkscs","cn-big5","csbig5","x-x-big5"],name:"Big5"}],heading:"Legacy multi-byte Chinese (traditional) encodings"},{encodings:[{labels:["cseucpkdfmtjapanese","euc-jp","x-euc-jp"],name:"EUC-JP"},{labels:["csiso2022jp","iso-2022-jp"],name:"ISO-2022-JP"},{labels:["csshiftjis","ms932","ms_kanji","shift-jis","shift_jis","sjis","windows-31j","x-sjis"],name:"Shift_JIS"}],heading:"Legacy multi-byte Japanese encodings"},{encodings:[{labels:["cseuckr","csksc56011987","euc-kr","iso-ir-149","korean","ks_c_5601-1987","ks_c_5601-1989","ksc5601","ksc_5601","windows-949"],name:"EUC-KR"}],heading:"Legacy multi-byte Korean encodings"},{encodings:[{labels:["csiso2022kr","hz-gb-2312","iso-2022-cn","iso-2022-cn-ext","iso-2022-kr"],name:"replacement"},{labels:["utf-16be"],name:"UTF-16BE"},{labels:["utf-16","utf-16le"],name:"UTF-16LE"},{labels:["x-user-defined"],name:"x-user-defined"}],heading:"Legacy miscellaneous encodings"}],h={}
d.forEach((function(n){n.encodings.forEach((function(n){n.labels.forEach((function(e){h[e]=n}))}))}))
var g,p,b={},_={}
function w(n,e){return e&&e[n]||null}function m(n,e){var r=e.indexOf(n)
return-1===r?null:r}function v(e){if(!("encoding-indexes"in n))throw Error("Indexes missing. Did you forget to include encoding-indexes-50f27403be5972eae4831f5b69db1f80.js first?")
return n["encoding-indexes"][e]}var y="utf-8"
function x(n,e){if(!(this instanceof x))throw TypeError("Called as a function. Did you forget 'new'?")
n=void 0!==n?String(n):y,e=i(e),this._encoding=null,this._decoder=null,this._ignoreBOM=!1,this._BOMseen=!1,this._error_mode="replacement",this._do_not_flush=!1
var r=c(n)
if(null===r||"replacement"===r.name)throw RangeError("Unknown encoding: "+n)
if(!_[r.name])throw Error("Decoder not present. Did you forget to include encoding-indexes-50f27403be5972eae4831f5b69db1f80.js first?")
var t=this
return t._encoding=r,Boolean(e.fatal)&&(t._error_mode="fatal"),Boolean(e.ignoreBOM)&&(t._ignoreBOM=!0),Object.defineProperty||(this.encoding=t._encoding.name.toLowerCase(),this.fatal="fatal"===t._error_mode,this.ignoreBOM=t._ignoreBOM),t}function O(e,r){if(!(this instanceof O))throw TypeError("Called as a function. Did you forget 'new'?")
r=i(r),this._encoding=null,this._encoder=null,this._do_not_flush=!1,this._fatal=Boolean(r.fatal)?"fatal":"replacement"
var t=this
if(Boolean(r.NONSTANDARD_allowLegacyEncoding)){var o=c(e=void 0!==e?String(e):y)
if(null===o||"replacement"===o.name)throw RangeError("Unknown encoding: "+e)
if(!b[o.name])throw Error("Encoder not present. Did you forget to include encoding-indexes-50f27403be5972eae4831f5b69db1f80.js first?")
t._encoding=o}else t._encoding=c("utf-8"),void 0!==e&&"console"in n&&console.warn("TextEncoder constructor called with encoding label, which is ignored.")
return Object.defineProperty||(this.encoding=t._encoding.name.toLowerCase()),t}function k(n){var r=n.fatal,i=0,t=0,o=0,a=128,f=191
this.handler=function(n,c){if(c===s&&0!==o)return o=0,l(r)
if(c===s)return u
if(0===o){if(e(c,0,127))return c
if(e(c,194,223))o=1,i=31&c
else if(e(c,224,239))224===c&&(a=160),237===c&&(f=159),o=2,i=15&c
else{if(!e(c,240,244))return l(r)
240===c&&(a=144),244===c&&(f=143),o=3,i=7&c}return null}if(!e(c,a,f))return i=o=t=0,a=128,f=191,n.prepend(c),l(r)
if(a=128,f=191,i=i<<6|63&c,(t+=1)!==o)return null
var d=i
return i=o=t=0,d}}function E(n){n.fatal
this.handler=function(n,r){if(r===s)return u
if(o(r))return r
var i,t
e(r,128,2047)?(i=1,t=192):e(r,2048,65535)?(i=2,t=224):e(r,65536,1114111)&&(i=3,t=240)
for(var a=[(r>>6*i)+t];i>0;){var l=r>>6*(i-1)
a.push(128|63&l),i-=1}return a}}function j(n,e){var r=e.fatal
this.handler=function(e,i){if(i===s)return u
if(t(i))return i
var o=n[i-128]
return null===o?l(r):o}}function B(n,e){e.fatal
this.handler=function(e,r){if(r===s)return u
if(o(r))return r
var i=m(r,n)
return null===i&&f(r),i+128}}function S(n){var r=n.fatal,i=0,o=0,a=0
this.handler=function(n,f){if(f===s&&0===i&&0===o&&0===a)return u
var c
if(f!==s||0===i&&0===o&&0===a||(i=0,o=0,a=0,l(r)),0!==a){c=null,e(f,48,57)&&(c=function(n){if(n>39419&&n<189e3||n>1237575)return null
if(7457===n)return 59335
var e,r=0,i=0,t=v("gb18030-ranges")
for(e=0;e<t.length;++e){var o=t[e]
if(!(o[0]<=n))break
r=o[0],i=o[1]}return i+n-r}(10*(126*(10*(i-129)+o-48)+a-129)+f-48))
var d=[o,a,f]
return i=0,o=0,a=0,null===c?(n.prepend(d),l(r)):c}if(0!==o)return e(f,129,254)?(a=f,null):(n.prepend([o,f]),i=0,o=0,l(r))
if(0!==i){if(e(f,48,57))return o=f,null
var h=i,g=null
i=0
var p=f<127?64:65
return(e(f,64,126)||e(f,128,254))&&(g=190*(h-129)+(f-p)),null===(c=null===g?null:w(g,v("gb18030")))&&t(f)&&n.prepend(f),null===c?l(r):c}return t(f)?f:128===f?8364:e(f,129,254)?(i=f,null):l(r)}}function T(n,e){n.fatal
this.handler=function(n,i){if(i===s)return u
if(o(i))return i
if(58853===i)return f(i)
if(e&&8364===i)return 128
var t=m(i,v("gb18030"))
if(null!==t){var a=t%190
return[r(t/190)+129,a+(a<63?64:65)]}if(e)return f(i)
t=function(n){if(59335===n)return 7457
var e,r=0,i=0,t=v("gb18030-ranges")
for(e=0;e<t.length;++e){var o=t[e]
if(!(o[1]<=n))break
r=o[1],i=o[0]}return i+n-r}(i)
var l=r(t/10/126/10),c=r((t-=10*l*126*10)/10/126),d=r((t-=10*c*126)/10)
return[l+129,c+48,d+129,t-10*d+48]}}function I(n){var r=n.fatal,i=0
this.handler=function(n,o){if(o===s&&0!==i)return i=0,l(r)
if(o===s&&0===i)return u
if(0!==i){var a=i,f=null
i=0
var c=o<127?64:98
switch((e(o,64,126)||e(o,161,254))&&(f=157*(a-129)+(o-c)),f){case 1133:return[202,772]
case 1135:return[202,780]
case 1164:return[234,772]
case 1166:return[234,780]}var d=null===f?null:w(f,v("big5"))
return null===d&&t(o)&&n.prepend(o),null===d?l(r):d}return t(o)?o:e(o,129,254)?(i=o,null):l(r)}}function U(n){n.fatal
this.handler=function(n,e){if(e===s)return u
if(o(e))return e
var i=function(n){p=p||v("big5").map((function(n,e){return e<5024?null:n}))
var e=p
return 9552===n||9566===n||9569===n||9578===n||21313===n||21317===n?e.lastIndexOf(n):m(n,e)}(e)
if(null===i)return f(e)
var t=r(i/157)+129
if(t<161)return f(e)
var a=i%157
return[t,a+(a<63?64:98)]}}function C(n){var r=n.fatal,i=!1,o=0
this.handler=function(n,a){if(a===s&&0!==o)return o=0,l(r)
if(a===s&&0===o)return u
if(142===o&&e(a,161,223))return o=0,65216+a
if(143===o&&e(a,161,254))return i=!0,o=a,null
if(0!==o){var f=o
o=0
var c=null
return e(f,161,254)&&e(a,161,254)&&(c=w(94*(f-161)+(a-161),v(i?"jis0212":"jis0208"))),i=!1,e(a,161,254)||n.prepend(a),null===c?l(r):c}return t(a)?a:142===a||143===a||e(a,161,254)?(o=a,null):l(r)}}function A(n){n.fatal
this.handler=function(n,i){if(i===s)return u
if(o(i))return i
if(165===i)return 92
if(8254===i)return 126
if(e(i,65377,65439))return[142,i-65377+161]
8722===i&&(i=65293)
var t=m(i,v("jis0208"))
return null===t?f(i):[r(t/94)+161,t%94+161]}}function L(n){var r=n.fatal,i=0,t=1,o=2,a=3,f=4,c=5,d=6,h=i,g=i,p=0,b=!1
this.handler=function(n,_){switch(h){default:case i:return 27===_?(h=c,null):e(_,0,127)&&14!==_&&15!==_&&27!==_?(b=!1,_):_===s?u:(b=!1,l(r))
case t:return 27===_?(h=c,null):92===_?(b=!1,165):126===_?(b=!1,8254):e(_,0,127)&&14!==_&&15!==_&&27!==_&&92!==_&&126!==_?(b=!1,_):_===s?u:(b=!1,l(r))
case o:return 27===_?(h=c,null):e(_,33,95)?(b=!1,65344+_):_===s?u:(b=!1,l(r))
case a:return 27===_?(h=c,null):e(_,33,126)?(b=!1,p=_,h=f,null):_===s?u:(b=!1,l(r))
case f:if(27===_)return h=c,l(r)
if(e(_,33,126)){h=a
var m=w(94*(p-33)+_-33,v("jis0208"))
return null===m?l(r):m}return _===s?(h=a,n.prepend(_),l(r)):(h=a,l(r))
case c:return 36===_||40===_?(p=_,h=d,null):(n.prepend(_),b=!1,h=g,l(r))
case d:var y=p
p=0
var x=null
if(40===y&&66===_&&(x=i),40===y&&74===_&&(x=t),40===y&&73===_&&(x=o),36!==y||64!==_&&66!==_||(x=a),null!==x){h=h=x
var O=b
return b=!0,O?l(r):null}return n.prepend([y,_]),b=!1,h=g,l(r)}}}function M(n){n.fatal
var e=0,i=1,t=2,a=e
this.handler=function(n,l){if(l===s&&a!==e)return n.prepend(l),a=e,[27,40,66]
if(l===s&&a===e)return u
if(!(a!==e&&a!==i||14!==l&&15!==l&&27!==l))return f(65533)
if(a===e&&o(l))return l
if(a===i&&(o(l)&&92!==l&&126!==l||165==l||8254==l)){if(o(l))return l
if(165===l)return 92
if(8254===l)return 126}if(o(l)&&a!==e)return n.prepend(l),a=e,[27,40,66]
if((165===l||8254===l)&&a!==i)return n.prepend(l),a=i,[27,40,74]
8722===l&&(l=65293)
var c=m(l,v("jis0208"))
return null===c?f(l):a!==t?(n.prepend(l),a=t,[27,36,66]):[r(c/94)+33,c%94+33]}}function P(n){var r=n.fatal,i=0
this.handler=function(n,o){if(o===s&&0!==i)return i=0,l(r)
if(o===s&&0===i)return u
if(0!==i){var a=i,f=null
i=0
var c=o<127?64:65,d=a<160?129:193
if((e(o,64,126)||e(o,128,252))&&(f=188*(a-d)+o-c),e(f,8836,10715))return 48508+f
var h=null===f?null:w(f,v("jis0208"))
return null===h&&t(o)&&n.prepend(o),null===h?l(r):h}return t(o)||128===o?o:e(o,161,223)?65216+o:e(o,129,159)||e(o,224,252)?(i=o,null):l(r)}}function D(n){n.fatal
this.handler=function(n,i){if(i===s)return u
if(o(i)||128===i)return i
if(165===i)return 92
if(8254===i)return 126
if(e(i,65377,65439))return i-65377+161
8722===i&&(i=65293)
var t=function(n){return g=g||v("jis0208").map((function(n,r){return e(r,8272,8835)?null:n})),g.indexOf(n)}(i)
if(null===t)return f(i)
var a=r(t/188),l=t%188
return[a+(a<31?129:193),l+(l<63?64:65)]}}function F(n){var r=n.fatal,i=0
this.handler=function(n,o){if(o===s&&0!==i)return i=0,l(r)
if(o===s&&0===i)return u
if(0!==i){var a=i,f=null
i=0,e(o,65,254)&&(f=190*(a-129)+(o-65))
var c=null===f?null:w(f,v("euc-kr"))
return null===f&&t(o)&&n.prepend(o),null===c?l(r):c}return t(o)?o:e(o,129,254)?(i=o,null):l(r)}}function J(n){n.fatal
this.handler=function(n,e){if(e===s)return u
if(o(e))return e
var i=m(e,v("euc-kr"))
return null===i?f(e):[r(i/190)+129,i%190+65]}}function K(n,e){var r=n>>8,i=255&n
return e?[r,i]:[i,r]}function R(n,r){var i=r.fatal,t=null,o=null
this.handler=function(r,a){if(a===s&&(null!==t||null!==o))return l(i)
if(a===s&&null===t&&null===o)return u
if(null===t)return t=a,null
var f
if(f=n?(t<<8)+a:(a<<8)+t,t=null,null!==o){var c=o
return o=null,e(f,56320,57343)?65536+1024*(c-55296)+(f-56320):(r.prepend(K(f,n)),l(i))}return e(f,55296,56319)?(o=f,null):e(f,56320,57343)?l(i):f}}function G(n,r){r.fatal
this.handler=function(r,i){if(i===s)return u
if(e(i,0,65535))return K(i,n)
var t=K(55296+(i-65536>>10),n),o=K(56320+(i-65536&1023),n)
return t.concat(o)}}function N(n){n.fatal
this.handler=function(n,e){return e===s?u:t(e)?e:63360+e-128}}function q(n){n.fatal
this.handler=function(n,r){return r===s?u:o(r)?r:e(r,63360,63487)?r-63360+128:f(r)}}Object.defineProperty&&(Object.defineProperty(x.prototype,"encoding",{get:function(){return this._encoding.name.toLowerCase()}}),Object.defineProperty(x.prototype,"fatal",{get:function(){return"fatal"===this._error_mode}}),Object.defineProperty(x.prototype,"ignoreBOM",{get:function(){return this._ignoreBOM}})),x.prototype.decode=function(n,e){var r
r="object"==typeof n&&n instanceof ArrayBuffer?new Uint8Array(n):"object"==typeof n&&"buffer"in n&&n.buffer instanceof ArrayBuffer?new Uint8Array(n.buffer,n.byteOffset,n.byteLength):new Uint8Array(0),e=i(e),this._do_not_flush||(this._decoder=_[this._encoding.name]({fatal:"fatal"===this._error_mode}),this._BOMseen=!1),this._do_not_flush=Boolean(e.stream)
for(var t,o=new a(r),l=[];;){var f=o.read()
if(f===s)break
if((t=this._decoder.handler(o,f))===u)break
null!==t&&(Array.isArray(t)?l.push.apply(l,t):l.push(t))}if(!this._do_not_flush){do{if((t=this._decoder.handler(o,o.read()))===u)break
null!==t&&(Array.isArray(t)?l.push.apply(l,t):l.push(t))}while(!o.endOfStream())
this._decoder=null}return function(n){var e,r
return e=["UTF-8","UTF-16LE","UTF-16BE"],r=this._encoding.name,-1===e.indexOf(r)||this._ignoreBOM||this._BOMseen||(n.length>0&&65279===n[0]?(this._BOMseen=!0,n.shift()):n.length>0&&(this._BOMseen=!0)),function(n){for(var e="",r=0;r<n.length;++r){var i=n[r]
i<=65535?e+=String.fromCharCode(i):(i-=65536,e+=String.fromCharCode(55296+(i>>10),56320+(1023&i)))}return e}(n)}.call(this,l)},Object.defineProperty&&Object.defineProperty(O.prototype,"encoding",{get:function(){return this._encoding.name.toLowerCase()}}),O.prototype.encode=function(n,e){n=void 0===n?"":String(n),e=i(e),this._do_not_flush||(this._encoder=b[this._encoding.name]({fatal:"fatal"===this._fatal})),this._do_not_flush=Boolean(e.stream)
for(var r,t=new a(function(n){for(var e=String(n),r=e.length,i=0,t=[];i<r;){var o=e.charCodeAt(i)
if(o<55296||o>57343)t.push(o)
else if(56320<=o&&o<=57343)t.push(65533)
else if(55296<=o&&o<=56319)if(i===r-1)t.push(65533)
else{var s=e.charCodeAt(i+1)
if(56320<=s&&s<=57343){var a=1023&o,u=1023&s
t.push(65536+(a<<10)+u),i+=1}else t.push(65533)}i+=1}return t}(n)),o=[];;){var l=t.read()
if(l===s)break
if((r=this._encoder.handler(t,l))===u)break
Array.isArray(r)?o.push.apply(o,r):o.push(r)}if(!this._do_not_flush){for(;(r=this._encoder.handler(t,t.read()))!==u;)Array.isArray(r)?o.push.apply(o,r):o.push(r)
this._encoder=null}return new Uint8Array(o)},b["UTF-8"]=function(n){return new E(n)},_["UTF-8"]=function(n){return new k(n)},"encoding-indexes"in n&&d.forEach((function(n){"Legacy single-byte encodings"===n.heading&&n.encodings.forEach((function(n){var e=n.name,r=v(e.toLowerCase())
_[e]=function(n){return new j(r,n)},b[e]=function(n){return new B(r,n)}}))})),_.GBK=function(n){return new S(n)},b.GBK=function(n){return new T(n,!0)},b.gb18030=function(n){return new T(n)},_.gb18030=function(n){return new S(n)},b.Big5=function(n){return new U(n)},_.Big5=function(n){return new I(n)},b["EUC-JP"]=function(n){return new A(n)},_["EUC-JP"]=function(n){return new C(n)},b["ISO-2022-JP"]=function(n){return new M(n)},_["ISO-2022-JP"]=function(n){return new L(n)},b.Shift_JIS=function(n){return new D(n)},_.Shift_JIS=function(n){return new P(n)},b["EUC-KR"]=function(n){return new J(n)},_["EUC-KR"]=function(n){return new F(n)},b["UTF-16BE"]=function(n){return new G(!0,n)},_["UTF-16BE"]=function(n){return new R(!0,n)},b["UTF-16LE"]=function(n){return new G(!1,n)},_["UTF-16LE"]=function(n){return new R(!1,n)},b["x-user-defined"]=function(n){return new q(n)},_["x-user-defined"]=function(n){return new N(n)},n.TextEncoder||(n.TextEncoder=O),n.TextDecoder||(n.TextDecoder=x),"undefined"!=typeof module&&module.exports&&(module.exports={TextEncoder:n.TextEncoder,TextDecoder:n.TextDecoder,EncodingIndexes:n["encoding-indexes"]})})(this||{})
