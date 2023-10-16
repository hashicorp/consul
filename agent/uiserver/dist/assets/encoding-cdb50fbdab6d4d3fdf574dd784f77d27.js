(function(n){"use strict"
function e(n,e,r){return e<=n&&n<=r}"undefined"!=typeof module&&module.exports&&!n["encoding-indexes"]&&(n["encoding-indexes"]=require("./encoding-indexes-75eea16b259716db4fd162ee283d2ae5.js")["encoding-indexes"])
var r=Math.floor
function i(n){if(void 0===n)return{}
if(n===Object(n))return n
throw TypeError("Could not convert argument to dictionary")}function t(n){return 0<=n&&n<=127}var o=t
function s(n){this.tokens=[].slice.call(n),this.tokens.reverse()}s.prototype={endOfStream:function(){return!this.tokens.length},read:function(){return this.tokens.length?this.tokens.pop():-1},prepend:function(n){if(Array.isArray(n))for(var e=n;e.length;)this.tokens.push(e.pop())
else this.tokens.push(n)},push:function(n){if(Array.isArray(n))for(var e=n;e.length;)this.tokens.unshift(e.shift())
else this.tokens.unshift(n)}}
function a(n,e){if(n)throw TypeError("Decoder error")
return e||65533}function u(n){throw TypeError("The code point "+n+" could not be encoded.")}function l(n){return n=String(n).trim().toLowerCase(),Object.prototype.hasOwnProperty.call(c,n)?c[n]:null}var f=[{encodings:[{labels:["unicode-1-1-utf-8","utf-8","utf8"],name:"UTF-8"}],heading:"The Encoding"},{encodings:[{labels:["866","cp866","csibm866","ibm866"],name:"IBM866"},{labels:["csisolatin2","iso-8859-2","iso-ir-101","iso8859-2","iso88592","iso_8859-2","iso_8859-2:1987","l2","latin2"],name:"ISO-8859-2"},{labels:["csisolatin3","iso-8859-3","iso-ir-109","iso8859-3","iso88593","iso_8859-3","iso_8859-3:1988","l3","latin3"],name:"ISO-8859-3"},{labels:["csisolatin4","iso-8859-4","iso-ir-110","iso8859-4","iso88594","iso_8859-4","iso_8859-4:1988","l4","latin4"],name:"ISO-8859-4"},{labels:["csisolatincyrillic","cyrillic","iso-8859-5","iso-ir-144","iso8859-5","iso88595","iso_8859-5","iso_8859-5:1988"],name:"ISO-8859-5"},{labels:["arabic","asmo-708","csiso88596e","csiso88596i","csisolatinarabic","ecma-114","iso-8859-6","iso-8859-6-e","iso-8859-6-i","iso-ir-127","iso8859-6","iso88596","iso_8859-6","iso_8859-6:1987"],name:"ISO-8859-6"},{labels:["csisolatingreek","ecma-118","elot_928","greek","greek8","iso-8859-7","iso-ir-126","iso8859-7","iso88597","iso_8859-7","iso_8859-7:1987","sun_eu_greek"],name:"ISO-8859-7"},{labels:["csiso88598e","csisolatinhebrew","hebrew","iso-8859-8","iso-8859-8-e","iso-ir-138","iso8859-8","iso88598","iso_8859-8","iso_8859-8:1988","visual"],name:"ISO-8859-8"},{labels:["csiso88598i","iso-8859-8-i","logical"],name:"ISO-8859-8-I"},{labels:["csisolatin6","iso-8859-10","iso-ir-157","iso8859-10","iso885910","l6","latin6"],name:"ISO-8859-10"},{labels:["iso-8859-13","iso8859-13","iso885913"],name:"ISO-8859-13"},{labels:["iso-8859-14","iso8859-14","iso885914"],name:"ISO-8859-14"},{labels:["csisolatin9","iso-8859-15","iso8859-15","iso885915","iso_8859-15","l9"],name:"ISO-8859-15"},{labels:["iso-8859-16"],name:"ISO-8859-16"},{labels:["cskoi8r","koi","koi8","koi8-r","koi8_r"],name:"KOI8-R"},{labels:["koi8-ru","koi8-u"],name:"KOI8-U"},{labels:["csmacintosh","mac","macintosh","x-mac-roman"],name:"macintosh"},{labels:["dos-874","iso-8859-11","iso8859-11","iso885911","tis-620","windows-874"],name:"windows-874"},{labels:["cp1250","windows-1250","x-cp1250"],name:"windows-1250"},{labels:["cp1251","windows-1251","x-cp1251"],name:"windows-1251"},{labels:["ansi_x3.4-1968","ascii","cp1252","cp819","csisolatin1","ibm819","iso-8859-1","iso-ir-100","iso8859-1","iso88591","iso_8859-1","iso_8859-1:1987","l1","latin1","us-ascii","windows-1252","x-cp1252"],name:"windows-1252"},{labels:["cp1253","windows-1253","x-cp1253"],name:"windows-1253"},{labels:["cp1254","csisolatin5","iso-8859-9","iso-ir-148","iso8859-9","iso88599","iso_8859-9","iso_8859-9:1989","l5","latin5","windows-1254","x-cp1254"],name:"windows-1254"},{labels:["cp1255","windows-1255","x-cp1255"],name:"windows-1255"},{labels:["cp1256","windows-1256","x-cp1256"],name:"windows-1256"},{labels:["cp1257","windows-1257","x-cp1257"],name:"windows-1257"},{labels:["cp1258","windows-1258","x-cp1258"],name:"windows-1258"},{labels:["x-mac-cyrillic","x-mac-ukrainian"],name:"x-mac-cyrillic"}],heading:"Legacy single-byte encodings"},{encodings:[{labels:["chinese","csgb2312","csiso58gb231280","gb2312","gb_2312","gb_2312-80","gbk","iso-ir-58","x-gbk"],name:"GBK"},{labels:["gb18030"],name:"gb18030"}],heading:"Legacy multi-byte Chinese (simplified) encodings"},{encodings:[{labels:["big5","big5-hkscs","cn-big5","csbig5","x-x-big5"],name:"Big5"}],heading:"Legacy multi-byte Chinese (traditional) encodings"},{encodings:[{labels:["cseucpkdfmtjapanese","euc-jp","x-euc-jp"],name:"EUC-JP"},{labels:["csiso2022jp","iso-2022-jp"],name:"ISO-2022-JP"},{labels:["csshiftjis","ms932","ms_kanji","shift-jis","shift_jis","sjis","windows-31j","x-sjis"],name:"Shift_JIS"}],heading:"Legacy multi-byte Japanese encodings"},{encodings:[{labels:["cseuckr","csksc56011987","euc-kr","iso-ir-149","korean","ks_c_5601-1987","ks_c_5601-1989","ksc5601","ksc_5601","windows-949"],name:"EUC-KR"}],heading:"Legacy multi-byte Korean encodings"},{encodings:[{labels:["csiso2022kr","hz-gb-2312","iso-2022-cn","iso-2022-cn-ext","iso-2022-kr"],name:"replacement"},{labels:["utf-16be"],name:"UTF-16BE"},{labels:["utf-16","utf-16le"],name:"UTF-16LE"},{labels:["x-user-defined"],name:"x-user-defined"}],heading:"Legacy miscellaneous encodings"}],c={}
f.forEach((function(n){n.encodings.forEach((function(n){n.labels.forEach((function(e){c[e]=n}))}))}))
var d,h,g={},p={}
function _(n,e){return e&&e[n]||null}function b(n,e){var r=e.indexOf(n)
return-1===r?null:r}function w(e){if(!("encoding-indexes"in n))throw Error("Indexes missing. Did you forget to include encoding-indexes-75eea16b259716db4fd162ee283d2ae5.js first?")
return n["encoding-indexes"][e]}function m(n,e){if(!(this instanceof m))throw TypeError("Called as a function. Did you forget 'new'?")
n=void 0!==n?String(n):"utf-8",e=i(e),this._encoding=null,this._decoder=null,this._ignoreBOM=!1,this._BOMseen=!1,this._error_mode="replacement",this._do_not_flush=!1
var r=l(n)
if(null===r||"replacement"===r.name)throw RangeError("Unknown encoding: "+n)
if(!p[r.name])throw Error("Decoder not present. Did you forget to include encoding-indexes-75eea16b259716db4fd162ee283d2ae5.js first?")
return this._encoding=r,Boolean(e.fatal)&&(this._error_mode="fatal"),Boolean(e.ignoreBOM)&&(this._ignoreBOM=!0),Object.defineProperty||(this.encoding=this._encoding.name.toLowerCase(),this.fatal="fatal"===this._error_mode,this.ignoreBOM=this._ignoreBOM),this}function v(e,r){if(!(this instanceof v))throw TypeError("Called as a function. Did you forget 'new'?")
r=i(r),this._encoding=null,this._encoder=null,this._do_not_flush=!1,this._fatal=Boolean(r.fatal)?"fatal":"replacement"
if(Boolean(r.NONSTANDARD_allowLegacyEncoding)){var t=l(e=void 0!==e?String(e):"utf-8")
if(null===t||"replacement"===t.name)throw RangeError("Unknown encoding: "+e)
if(!g[t.name])throw Error("Encoder not present. Did you forget to include encoding-indexes-75eea16b259716db4fd162ee283d2ae5.js first?")
this._encoding=t}else this._encoding=l("utf-8"),void 0!==e&&"console"in n&&console.warn("TextEncoder constructor called with encoding label, which is ignored.")
return Object.defineProperty||(this.encoding=this._encoding.name.toLowerCase()),this}function y(n){var r=n.fatal,i=0,t=0,o=0,s=128,u=191
this.handler=function(n,l){if(-1===l&&0!==o)return o=0,a(r)
if(-1===l)return-1
if(0===o){if(e(l,0,127))return l
if(e(l,194,223))o=1,i=31&l
else if(e(l,224,239))224===l&&(s=160),237===l&&(u=159),o=2,i=15&l
else{if(!e(l,240,244))return a(r)
240===l&&(s=144),244===l&&(u=143),o=3,i=7&l}return null}if(!e(l,s,u))return i=o=t=0,s=128,u=191,n.prepend(l),a(r)
if(s=128,u=191,i=i<<6|63&l,(t+=1)!==o)return null
var f=i
return i=o=t=0,f}}function x(n){n.fatal
this.handler=function(n,r){if(-1===r)return-1
if(o(r))return r
var i,t
e(r,128,2047)?(i=1,t=192):e(r,2048,65535)?(i=2,t=224):e(r,65536,1114111)&&(i=3,t=240)
for(var s=[(r>>6*i)+t];i>0;){var a=r>>6*(i-1)
s.push(128|63&a),i-=1}return s}}function O(n,e){var r=e.fatal
this.handler=function(e,i){if(-1===i)return-1
if(t(i))return i
var o=n[i-128]
return null===o?a(r):o}}function k(n,e){e.fatal
this.handler=function(e,r){if(-1===r)return-1
if(o(r))return r
var i=b(r,n)
return null===i&&u(r),i+128}}function E(n){var r=n.fatal,i=0,o=0,s=0
this.handler=function(n,u){if(-1===u&&0===i&&0===o&&0===s)return-1
var l
if(-1!==u||0===i&&0===o&&0===s||(i=0,o=0,s=0,a(r)),0!==s){l=null,e(u,48,57)&&(l=function(n){if(n>39419&&n<189e3||n>1237575)return null
if(7457===n)return 59335
var e,r=0,i=0,t=w("gb18030-ranges")
for(e=0;e<t.length;++e){var o=t[e]
if(!(o[0]<=n))break
r=o[0],i=o[1]}return i+n-r}(10*(126*(10*(i-129)+o-48)+s-129)+u-48))
var f=[o,s,u]
return i=0,o=0,s=0,null===l?(n.prepend(f),a(r)):l}if(0!==o)return e(u,129,254)?(s=u,null):(n.prepend([o,u]),i=0,o=0,a(r))
if(0!==i){if(e(u,48,57))return o=u,null
var c=i,d=null
i=0
var h=u<127?64:65
return(e(u,64,126)||e(u,128,254))&&(d=190*(c-129)+(u-h)),null===(l=null===d?null:_(d,w("gb18030")))&&t(u)&&n.prepend(u),null===l?a(r):l}return t(u)?u:128===u?8364:e(u,129,254)?(i=u,null):a(r)}}function j(n,e){n.fatal
this.handler=function(n,i){if(-1===i)return-1
if(o(i))return i
if(58853===i)return u(i)
if(e&&8364===i)return 128
var t=b(i,w("gb18030"))
if(null!==t){var s=t%190
return[r(t/190)+129,s+(s<63?64:65)]}if(e)return u(i)
t=function(n){if(59335===n)return 7457
var e,r=0,i=0,t=w("gb18030-ranges")
for(e=0;e<t.length;++e){var o=t[e]
if(!(o[1]<=n))break
r=o[1],i=o[0]}return i+n-r}(i)
var a=r(t/10/126/10),l=r((t-=10*a*126*10)/10/126),f=r((t-=10*l*126)/10)
return[a+129,l+48,f+129,t-10*f+48]}}function B(n){var r=n.fatal,i=0
this.handler=function(n,o){if(-1===o&&0!==i)return i=0,a(r)
if(-1===o&&0===i)return-1
if(0!==i){var s=i,u=null
i=0
var l=o<127?64:98
switch((e(o,64,126)||e(o,161,254))&&(u=157*(s-129)+(o-l)),u){case 1133:return[202,772]
case 1135:return[202,780]
case 1164:return[234,772]
case 1166:return[234,780]}var f=null===u?null:_(u,w("big5"))
return null===f&&t(o)&&n.prepend(o),null===f?a(r):f}return t(o)?o:e(o,129,254)?(i=o,null):a(r)}}function S(n){n.fatal
this.handler=function(n,e){if(-1===e)return-1
if(o(e))return e
var i=function(n){var e=h=h||w("big5").map((function(n,e){return e<5024?null:n}))
return 9552===n||9566===n||9569===n||9578===n||21313===n||21317===n?e.lastIndexOf(n):b(n,e)}(e)
if(null===i)return u(e)
var t=r(i/157)+129
if(t<161)return u(e)
var s=i%157
return[t,s+(s<63?64:98)]}}function T(n){var r=n.fatal,i=!1,o=0
this.handler=function(n,s){if(-1===s&&0!==o)return o=0,a(r)
if(-1===s&&0===o)return-1
if(142===o&&e(s,161,223))return o=0,65216+s
if(143===o&&e(s,161,254))return i=!0,o=s,null
if(0!==o){var u=o
o=0
var l=null
return e(u,161,254)&&e(s,161,254)&&(l=_(94*(u-161)+(s-161),w(i?"jis0212":"jis0208"))),i=!1,e(s,161,254)||n.prepend(s),null===l?a(r):l}return t(s)?s:142===s||143===s||e(s,161,254)?(o=s,null):a(r)}}function I(n){n.fatal
this.handler=function(n,i){if(-1===i)return-1
if(o(i))return i
if(165===i)return 92
if(8254===i)return 126
if(e(i,65377,65439))return[142,i-65377+161]
8722===i&&(i=65293)
var t=b(i,w("jis0208"))
return null===t?u(i):[r(t/94)+161,t%94+161]}}function U(n){var r=n.fatal,i=0,t=1,o=2,s=3,u=4,l=5,f=6,c=i,d=i,h=0,g=!1
this.handler=function(n,p){switch(c){default:case i:return 27===p?(c=l,null):e(p,0,127)&&14!==p&&15!==p&&27!==p?(g=!1,p):-1===p?-1:(g=!1,a(r))
case t:return 27===p?(c=l,null):92===p?(g=!1,165):126===p?(g=!1,8254):e(p,0,127)&&14!==p&&15!==p&&27!==p&&92!==p&&126!==p?(g=!1,p):-1===p?-1:(g=!1,a(r))
case o:return 27===p?(c=l,null):e(p,33,95)?(g=!1,65344+p):-1===p?-1:(g=!1,a(r))
case s:return 27===p?(c=l,null):e(p,33,126)?(g=!1,h=p,c=u,null):-1===p?-1:(g=!1,a(r))
case u:if(27===p)return c=l,a(r)
if(e(p,33,126)){c=s
var b=_(94*(h-33)+p-33,w("jis0208"))
return null===b?a(r):b}return-1===p?(c=s,n.prepend(p),a(r)):(c=s,a(r))
case l:return 36===p||40===p?(h=p,c=f,null):(n.prepend(p),g=!1,c=d,a(r))
case f:var m=h
h=0
var v=null
if(40===m&&66===p&&(v=i),40===m&&74===p&&(v=t),40===m&&73===p&&(v=o),36!==m||64!==p&&66!==p||(v=s),null!==v){c=c=v
var y=g
return g=!0,y?a(r):null}return n.prepend([m,p]),g=!1,c=d,a(r)}}}function C(n){n.fatal
var e=0,i=1,t=2,s=e
this.handler=function(n,a){if(-1===a&&s!==e)return n.prepend(a),s=e,[27,40,66]
if(-1===a&&s===e)return-1
if(!(s!==e&&s!==i||14!==a&&15!==a&&27!==a))return u(65533)
if(s===e&&o(a))return a
if(s===i&&(o(a)&&92!==a&&126!==a||165==a||8254==a)){if(o(a))return a
if(165===a)return 92
if(8254===a)return 126}if(o(a)&&s!==e)return n.prepend(a),s=e,[27,40,66]
if((165===a||8254===a)&&s!==i)return n.prepend(a),s=i,[27,40,74]
8722===a&&(a=65293)
var l=b(a,w("jis0208"))
return null===l?u(a):s!==t?(n.prepend(a),s=t,[27,36,66]):[r(l/94)+33,l%94+33]}}function A(n){var r=n.fatal,i=0
this.handler=function(n,o){if(-1===o&&0!==i)return i=0,a(r)
if(-1===o&&0===i)return-1
if(0!==i){var s=i,u=null
i=0
var l=o<127?64:65,f=s<160?129:193
if((e(o,64,126)||e(o,128,252))&&(u=188*(s-f)+o-l),e(u,8836,10715))return 48508+u
var c=null===u?null:_(u,w("jis0208"))
return null===c&&t(o)&&n.prepend(o),null===c?a(r):c}return t(o)||128===o?o:e(o,161,223)?65216+o:e(o,129,159)||e(o,224,252)?(i=o,null):a(r)}}function L(n){n.fatal
this.handler=function(n,i){if(-1===i)return-1
if(o(i)||128===i)return i
if(165===i)return 92
if(8254===i)return 126
if(e(i,65377,65439))return i-65377+161
8722===i&&(i=65293)
var t=function(n){return(d=d||w("jis0208").map((function(n,r){return e(r,8272,8835)?null:n}))).indexOf(n)}(i)
if(null===t)return u(i)
var s=r(t/188),a=t%188
return[s+(s<31?129:193),a+(a<63?64:65)]}}function M(n){var r=n.fatal,i=0
this.handler=function(n,o){if(-1===o&&0!==i)return i=0,a(r)
if(-1===o&&0===i)return-1
if(0!==i){var s=i,u=null
i=0,e(o,65,254)&&(u=190*(s-129)+(o-65))
var l=null===u?null:_(u,w("euc-kr"))
return null===u&&t(o)&&n.prepend(o),null===l?a(r):l}return t(o)?o:e(o,129,254)?(i=o,null):a(r)}}function P(n){n.fatal
this.handler=function(n,e){if(-1===e)return-1
if(o(e))return e
var i=b(e,w("euc-kr"))
return null===i?u(e):[r(i/190)+129,i%190+65]}}function D(n,e){var r=n>>8,i=255&n
return e?[r,i]:[i,r]}function F(n,r){var i=r.fatal,t=null,o=null
this.handler=function(r,s){if(-1===s&&(null!==t||null!==o))return a(i)
if(-1===s&&null===t&&null===o)return-1
if(null===t)return t=s,null
var u
if(u=n?(t<<8)+s:(s<<8)+t,t=null,null!==o){var l=o
return o=null,e(u,56320,57343)?65536+1024*(l-55296)+(u-56320):(r.prepend(D(u,n)),a(i))}return e(u,55296,56319)?(o=u,null):e(u,56320,57343)?a(i):u}}function J(n,r){r.fatal
this.handler=function(r,i){if(-1===i)return-1
if(e(i,0,65535))return D(i,n)
var t=D(55296+(i-65536>>10),n),o=D(56320+(i-65536&1023),n)
return t.concat(o)}}function K(n){n.fatal
this.handler=function(n,e){return-1===e?-1:t(e)?e:63360+e-128}}function R(n){n.fatal
this.handler=function(n,r){return-1===r?-1:o(r)?r:e(r,63360,63487)?r-63360+128:u(r)}}Object.defineProperty&&(Object.defineProperty(m.prototype,"encoding",{get:function(){return this._encoding.name.toLowerCase()}}),Object.defineProperty(m.prototype,"fatal",{get:function(){return"fatal"===this._error_mode}}),Object.defineProperty(m.prototype,"ignoreBOM",{get:function(){return this._ignoreBOM}})),m.prototype.decode=function(n,e){var r
r="object"==typeof n&&n instanceof ArrayBuffer?new Uint8Array(n):"object"==typeof n&&"buffer"in n&&n.buffer instanceof ArrayBuffer?new Uint8Array(n.buffer,n.byteOffset,n.byteLength):new Uint8Array(0),e=i(e),this._do_not_flush||(this._decoder=p[this._encoding.name]({fatal:"fatal"===this._error_mode}),this._BOMseen=!1),this._do_not_flush=Boolean(e.stream)
for(var t,o=new s(r),a=[];;){var u=o.read()
if(-1===u)break
if(-1===(t=this._decoder.handler(o,u)))break
null!==t&&(Array.isArray(t)?a.push.apply(a,t):a.push(t))}if(!this._do_not_flush){do{if(-1===(t=this._decoder.handler(o,o.read())))break
null!==t&&(Array.isArray(t)?a.push.apply(a,t):a.push(t))}while(!o.endOfStream())
this._decoder=null}return function(n){var e,r
return e=["UTF-8","UTF-16LE","UTF-16BE"],r=this._encoding.name,-1===e.indexOf(r)||this._ignoreBOM||this._BOMseen||(n.length>0&&65279===n[0]?(this._BOMseen=!0,n.shift()):n.length>0&&(this._BOMseen=!0)),function(n){for(var e="",r=0;r<n.length;++r){var i=n[r]
i<=65535?e+=String.fromCharCode(i):(i-=65536,e+=String.fromCharCode(55296+(i>>10),56320+(1023&i)))}return e}(n)}.call(this,a)},Object.defineProperty&&Object.defineProperty(v.prototype,"encoding",{get:function(){return this._encoding.name.toLowerCase()}}),v.prototype.encode=function(n,e){n=void 0===n?"":String(n),e=i(e),this._do_not_flush||(this._encoder=g[this._encoding.name]({fatal:"fatal"===this._fatal})),this._do_not_flush=Boolean(e.stream)
for(var r,t=new s(function(n){for(var e=String(n),r=e.length,i=0,t=[];i<r;){var o=e.charCodeAt(i)
if(o<55296||o>57343)t.push(o)
else if(56320<=o&&o<=57343)t.push(65533)
else if(55296<=o&&o<=56319)if(i===r-1)t.push(65533)
else{var s=e.charCodeAt(i+1)
if(56320<=s&&s<=57343){var a=1023&o,u=1023&s
t.push(65536+(a<<10)+u),i+=1}else t.push(65533)}i+=1}return t}(n)),o=[];;){var a=t.read()
if(-1===a)break
if(-1===(r=this._encoder.handler(t,a)))break
Array.isArray(r)?o.push.apply(o,r):o.push(r)}if(!this._do_not_flush){for(;-1!==(r=this._encoder.handler(t,t.read()));)Array.isArray(r)?o.push.apply(o,r):o.push(r)
this._encoder=null}return new Uint8Array(o)},g["UTF-8"]=function(n){return new x(n)},p["UTF-8"]=function(n){return new y(n)},"encoding-indexes"in n&&f.forEach((function(n){"Legacy single-byte encodings"===n.heading&&n.encodings.forEach((function(n){var e=n.name,r=w(e.toLowerCase())
p[e]=function(n){return new O(r,n)},g[e]=function(n){return new k(r,n)}}))})),p.GBK=function(n){return new E(n)},g.GBK=function(n){return new j(n,!0)},g.gb18030=function(n){return new j(n)},p.gb18030=function(n){return new E(n)},g.Big5=function(n){return new S(n)},p.Big5=function(n){return new B(n)},g["EUC-JP"]=function(n){return new I(n)},p["EUC-JP"]=function(n){return new T(n)},g["ISO-2022-JP"]=function(n){return new C(n)},p["ISO-2022-JP"]=function(n){return new U(n)},g.Shift_JIS=function(n){return new L(n)},p.Shift_JIS=function(n){return new A(n)},g["EUC-KR"]=function(n){return new P(n)},p["EUC-KR"]=function(n){return new M(n)},g["UTF-16BE"]=function(n){return new J(!0,n)},p["UTF-16BE"]=function(n){return new F(!0,n)},g["UTF-16LE"]=function(n){return new J(!1,n)},p["UTF-16LE"]=function(n){return new F(!1,n)},g["x-user-defined"]=function(n){return new R(n)},p["x-user-defined"]=function(n){return new K(n)},n.TextEncoder||(n.TextEncoder=v),n.TextDecoder||(n.TextDecoder=m),"undefined"!=typeof module&&module.exports&&(module.exports={TextEncoder:n.TextEncoder,TextDecoder:n.TextDecoder,EncodingIndexes:n["encoding-indexes"]})})(this||{})
