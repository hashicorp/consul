/*! js-yaml 4.1.0 https://github.com/nodeca/js-yaml @license MIT */
(function(e,t){"object"==typeof exports&&"undefined"!=typeof module?t(exports):"function"==typeof define&&define.amd?define(["exports"],t):t((e="undefined"!=typeof globalThis?globalThis:e||self).jsyaml={})})(this,(function(e){"use strict"
function t(e){return null==e}var n={isNothing:t,isObject:function(e){return"object"==typeof e&&null!==e},toArray:function(e){return Array.isArray(e)?e:t(e)?[]:[e]},repeat:function(e,t){var n,i=""
for(n=0;n<t;n+=1)i+=e
return i},isNegativeZero:function(e){return 0===e&&Number.NEGATIVE_INFINITY===1/e},extend:function(e,t){var n,i,r,o
if(t)for(n=0,i=(o=Object.keys(t)).length;n<i;n+=1)e[r=o[n]]=t[r]
return e}}
function i(e,t){var n="",i=e.reason||"(unknown reason)"
return e.mark?(e.mark.name&&(n+='in "'+e.mark.name+'" '),n+="("+(e.mark.line+1)+":"+(e.mark.column+1)+")",!t&&e.mark.snippet&&(n+="\n\n"+e.mark.snippet),i+" "+n):i}function r(e,t){Error.call(this),this.name="YAMLException",this.reason=e,this.mark=t,this.message=i(this,!1),Error.captureStackTrace?Error.captureStackTrace(this,this.constructor):this.stack=(new Error).stack||""}r.prototype=Object.create(Error.prototype),r.prototype.constructor=r,r.prototype.toString=function(e){return this.name+": "+i(this,e)}
var o=r
function a(e,t,n,i,r){var o="",a="",l=Math.floor(r/2)-1
return i-t>l&&(t=i-l+(o=" ... ").length),n-i>l&&(n=i+l-(a=" ...").length),{str:o+e.slice(t,n).replace(/\t/g,"→")+a,pos:i-t+o.length}}function l(e,t){return n.repeat(" ",t-e.length)+e}var s=function(e,t){if(t=Object.create(t||null),!e.buffer)return null
t.maxLength||(t.maxLength=79),"number"!=typeof t.indent&&(t.indent=1),"number"!=typeof t.linesBefore&&(t.linesBefore=3),"number"!=typeof t.linesAfter&&(t.linesAfter=2)
for(var i,r=/\r?\n|\r|\0/g,o=[0],s=[],c=-1;i=r.exec(e.buffer);)s.push(i.index),o.push(i.index+i[0].length),e.position<=i.index&&c<0&&(c=o.length-2)
c<0&&(c=o.length-1)
var u,p,f="",d=Math.min(e.line+t.linesAfter,s.length).toString().length,h=t.maxLength-(t.indent+d+3)
for(u=1;u<=t.linesBefore&&!(c-u<0);u++)p=a(e.buffer,o[c-u],s[c-u],e.position-(o[c]-o[c-u]),h),f=n.repeat(" ",t.indent)+l((e.line-u+1).toString(),d)+" | "+p.str+"\n"+f
for(p=a(e.buffer,o[c],s[c],e.position,h),f+=n.repeat(" ",t.indent)+l((e.line+1).toString(),d)+" | "+p.str+"\n",f+=n.repeat("-",t.indent+d+3+p.pos)+"^\n",u=1;u<=t.linesAfter&&!(c+u>=s.length);u++)p=a(e.buffer,o[c+u],s[c+u],e.position-(o[c]-o[c+u]),h),f+=n.repeat(" ",t.indent)+l((e.line+u+1).toString(),d)+" | "+p.str+"\n"
return f.replace(/\n$/,"")},c=["kind","multi","resolve","construct","instanceOf","predicate","represent","representName","defaultStyle","styleAliases"],u=["scalar","sequence","mapping"]
var p=function(e,t){if(t=t||{},Object.keys(t).forEach((function(t){if(-1===c.indexOf(t))throw new o('Unknown option "'+t+'" is met in definition of "'+e+'" YAML type.')})),this.options=t,this.tag=e,this.kind=t.kind||null,this.resolve=t.resolve||function(){return!0},this.construct=t.construct||function(e){return e},this.instanceOf=t.instanceOf||null,this.predicate=t.predicate||null,this.represent=t.represent||null,this.representName=t.representName||null,this.defaultStyle=t.defaultStyle||null,this.multi=t.multi||!1,this.styleAliases=function(e){var t={}
return null!==e&&Object.keys(e).forEach((function(n){e[n].forEach((function(e){t[String(e)]=n}))})),t}(t.styleAliases||null),-1===u.indexOf(this.kind))throw new o('Unknown kind "'+this.kind+'" is specified for "'+e+'" YAML type.')}
function f(e,t){var n=[]
return e[t].forEach((function(e){var t=n.length
n.forEach((function(n,i){n.tag===e.tag&&n.kind===e.kind&&n.multi===e.multi&&(t=i)})),n[t]=e})),n}function d(e){return this.extend(e)}d.prototype.extend=function(e){var t=[],n=[]
if(e instanceof p)n.push(e)
else if(Array.isArray(e))n=n.concat(e)
else{if(!e||!Array.isArray(e.implicit)&&!Array.isArray(e.explicit))throw new o("Schema.extend argument should be a Type, [ Type ], or a schema definition ({ implicit: [...], explicit: [...] })")
e.implicit&&(t=t.concat(e.implicit)),e.explicit&&(n=n.concat(e.explicit))}t.forEach((function(e){if(!(e instanceof p))throw new o("Specified list of YAML types (or a single Type object) contains a non-Type object.")
if(e.loadKind&&"scalar"!==e.loadKind)throw new o("There is a non-scalar type in the implicit list of a schema. Implicit resolving of such types is not supported.")
if(e.multi)throw new o("There is a multi type in the implicit list of a schema. Multi tags can only be listed as explicit.")})),n.forEach((function(e){if(!(e instanceof p))throw new o("Specified list of YAML types (or a single Type object) contains a non-Type object.")}))
var i=Object.create(d.prototype)
return i.implicit=(this.implicit||[]).concat(t),i.explicit=(this.explicit||[]).concat(n),i.compiledImplicit=f(i,"implicit"),i.compiledExplicit=f(i,"explicit"),i.compiledTypeMap=function(){var e,t,n={scalar:{},sequence:{},mapping:{},fallback:{},multi:{scalar:[],sequence:[],mapping:[],fallback:[]}}
function i(e){e.multi?(n.multi[e.kind].push(e),n.multi.fallback.push(e)):n[e.kind][e.tag]=n.fallback[e.tag]=e}for(e=0,t=arguments.length;e<t;e+=1)arguments[e].forEach(i)
return n}(i.compiledImplicit,i.compiledExplicit),i}
var h=d,m=new p("tag:yaml.org,2002:str",{kind:"scalar",construct:function(e){return null!==e?e:""}}),g=new p("tag:yaml.org,2002:seq",{kind:"sequence",construct:function(e){return null!==e?e:[]}}),y=new p("tag:yaml.org,2002:map",{kind:"mapping",construct:function(e){return null!==e?e:{}}}),b=new h({explicit:[m,g,y]})
var A=new p("tag:yaml.org,2002:null",{kind:"scalar",resolve:function(e){if(null===e)return!0
var t=e.length
return 1===t&&"~"===e||4===t&&("null"===e||"Null"===e||"NULL"===e)},construct:function(){return null},predicate:function(e){return null===e},represent:{canonical:function(){return"~"},lowercase:function(){return"null"},uppercase:function(){return"NULL"},camelcase:function(){return"Null"},empty:function(){return""}},defaultStyle:"lowercase"})
var v=new p("tag:yaml.org,2002:bool",{kind:"scalar",resolve:function(e){if(null===e)return!1
var t=e.length
return 4===t&&("true"===e||"True"===e||"TRUE"===e)||5===t&&("false"===e||"False"===e||"FALSE"===e)},construct:function(e){return"true"===e||"True"===e||"TRUE"===e},predicate:function(e){return"[object Boolean]"===Object.prototype.toString.call(e)},represent:{lowercase:function(e){return e?"true":"false"},uppercase:function(e){return e?"TRUE":"FALSE"},camelcase:function(e){return e?"True":"False"}},defaultStyle:"lowercase"})
function k(e){return 48<=e&&e<=55}function w(e){return 48<=e&&e<=57}var C=new p("tag:yaml.org,2002:int",{kind:"scalar",resolve:function(e){if(null===e)return!1
var t,n,i=e.length,r=0,o=!1
if(!i)return!1
if("-"!==(t=e[r])&&"+"!==t||(t=e[++r]),"0"===t){if(r+1===i)return!0
if("b"===(t=e[++r])){for(r++;r<i;r++)if("_"!==(t=e[r])){if("0"!==t&&"1"!==t)return!1
o=!0}return o&&"_"!==t}if("x"===t){for(r++;r<i;r++)if("_"!==(t=e[r])){if(!(48<=(n=e.charCodeAt(r))&&n<=57||65<=n&&n<=70||97<=n&&n<=102))return!1
o=!0}return o&&"_"!==t}if("o"===t){for(r++;r<i;r++)if("_"!==(t=e[r])){if(!k(e.charCodeAt(r)))return!1
o=!0}return o&&"_"!==t}}if("_"===t)return!1
for(;r<i;r++)if("_"!==(t=e[r])){if(!w(e.charCodeAt(r)))return!1
o=!0}return!(!o||"_"===t)},construct:function(e){var t,n=e,i=1
if(-1!==n.indexOf("_")&&(n=n.replace(/_/g,"")),"-"!==(t=n[0])&&"+"!==t||("-"===t&&(i=-1),t=(n=n.slice(1))[0]),"0"===n)return 0
if("0"===t){if("b"===n[1])return i*parseInt(n.slice(2),2)
if("x"===n[1])return i*parseInt(n.slice(2),16)
if("o"===n[1])return i*parseInt(n.slice(2),8)}return i*parseInt(n,10)},predicate:function(e){return"[object Number]"===Object.prototype.toString.call(e)&&e%1==0&&!n.isNegativeZero(e)},represent:{binary:function(e){return e>=0?"0b"+e.toString(2):"-0b"+e.toString(2).slice(1)},octal:function(e){return e>=0?"0o"+e.toString(8):"-0o"+e.toString(8).slice(1)},decimal:function(e){return e.toString(10)},hexadecimal:function(e){return e>=0?"0x"+e.toString(16).toUpperCase():"-0x"+e.toString(16).toUpperCase().slice(1)}},defaultStyle:"decimal",styleAliases:{binary:[2,"bin"],octal:[8,"oct"],decimal:[10,"dec"],hexadecimal:[16,"hex"]}}),x=new RegExp("^(?:[-+]?(?:[0-9][0-9_]*)(?:\\.[0-9_]*)?(?:[eE][-+]?[0-9]+)?|\\.[0-9_]+(?:[eE][-+]?[0-9]+)?|[-+]?\\.(?:inf|Inf|INF)|\\.(?:nan|NaN|NAN))$")
var I=/^[-+]?[0-9]+e/
var S=new p("tag:yaml.org,2002:float",{kind:"scalar",resolve:function(e){return null!==e&&!(!x.test(e)||"_"===e[e.length-1])},construct:function(e){var t,n
return n="-"===(t=e.replace(/_/g,"").toLowerCase())[0]?-1:1,"+-".indexOf(t[0])>=0&&(t=t.slice(1)),".inf"===t?1===n?Number.POSITIVE_INFINITY:Number.NEGATIVE_INFINITY:".nan"===t?NaN:n*parseFloat(t,10)},predicate:function(e){return"[object Number]"===Object.prototype.toString.call(e)&&(e%1!=0||n.isNegativeZero(e))},represent:function(e,t){var i
if(isNaN(e))switch(t){case"lowercase":return".nan"
case"uppercase":return".NAN"
case"camelcase":return".NaN"}else if(Number.POSITIVE_INFINITY===e)switch(t){case"lowercase":return".inf"
case"uppercase":return".INF"
case"camelcase":return".Inf"}else if(Number.NEGATIVE_INFINITY===e)switch(t){case"lowercase":return"-.inf"
case"uppercase":return"-.INF"
case"camelcase":return"-.Inf"}else if(n.isNegativeZero(e))return"-0.0"
return i=e.toString(10),I.test(i)?i.replace("e",".e"):i},defaultStyle:"lowercase"}),j=b.extend({implicit:[A,v,C,S]}),O=j,T=new RegExp("^([0-9][0-9][0-9][0-9])-([0-9][0-9])-([0-9][0-9])$"),E=new RegExp("^([0-9][0-9][0-9][0-9])-([0-9][0-9]?)-([0-9][0-9]?)(?:[Tt]|[ \\t]+)([0-9][0-9]?):([0-9][0-9]):([0-9][0-9])(?:\\.([0-9]*))?(?:[ \\t]*(Z|([-+])([0-9][0-9]?)(?::([0-9][0-9]))?))?$")
var M=new p("tag:yaml.org,2002:timestamp",{kind:"scalar",resolve:function(e){return null!==e&&(null!==T.exec(e)||null!==E.exec(e))},construct:function(e){var t,n,i,r,o,a,l,s,c=0,u=null
if(null===(t=T.exec(e))&&(t=E.exec(e)),null===t)throw new Error("Date resolve error")
if(n=+t[1],i=+t[2]-1,r=+t[3],!t[4])return new Date(Date.UTC(n,i,r))
if(o=+t[4],a=+t[5],l=+t[6],t[7]){for(c=t[7].slice(0,3);c.length<3;)c+="0"
c=+c}return t[9]&&(u=6e4*(60*+t[10]+ +(t[11]||0)),"-"===t[9]&&(u=-u)),s=new Date(Date.UTC(n,i,r,o,a,l,c)),u&&s.setTime(s.getTime()-u),s},instanceOf:Date,represent:function(e){return e.toISOString()}})
var L=new p("tag:yaml.org,2002:merge",{kind:"scalar",resolve:function(e){return"<<"===e||null===e}}),N="ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/=\n\r"
var F=new p("tag:yaml.org,2002:binary",{kind:"scalar",resolve:function(e){if(null===e)return!1
var t,n,i=0,r=e.length,o=N
for(n=0;n<r;n++)if(!((t=o.indexOf(e.charAt(n)))>64)){if(t<0)return!1
i+=6}return i%8==0},construct:function(e){var t,n,i=e.replace(/[\r\n=]/g,""),r=i.length,o=N,a=0,l=[]
for(t=0;t<r;t++)t%4==0&&t&&(l.push(a>>16&255),l.push(a>>8&255),l.push(255&a)),a=a<<6|o.indexOf(i.charAt(t))
return 0===(n=r%4*6)?(l.push(a>>16&255),l.push(a>>8&255),l.push(255&a)):18===n?(l.push(a>>10&255),l.push(a>>2&255)):12===n&&l.push(a>>4&255),new Uint8Array(l)},predicate:function(e){return"[object Uint8Array]"===Object.prototype.toString.call(e)},represent:function(e){var t,n,i="",r=0,o=e.length,a=N
for(t=0;t<o;t++)t%3==0&&t&&(i+=a[r>>18&63],i+=a[r>>12&63],i+=a[r>>6&63],i+=a[63&r]),r=(r<<8)+e[t]
return 0===(n=o%3)?(i+=a[r>>18&63],i+=a[r>>12&63],i+=a[r>>6&63],i+=a[63&r]):2===n?(i+=a[r>>10&63],i+=a[r>>4&63],i+=a[r<<2&63],i+=a[64]):1===n&&(i+=a[r>>2&63],i+=a[r<<4&63],i+=a[64],i+=a[64]),i}}),_=Object.prototype.hasOwnProperty,D=Object.prototype.toString
var q=new p("tag:yaml.org,2002:omap",{kind:"sequence",resolve:function(e){if(null===e)return!0
var t,n,i,r,o,a=[],l=e
for(t=0,n=l.length;t<n;t+=1){if(i=l[t],o=!1,"[object Object]"!==D.call(i))return!1
for(r in i)if(_.call(i,r)){if(o)return!1
o=!0}if(!o)return!1
if(-1!==a.indexOf(r))return!1
a.push(r)}return!0},construct:function(e){return null!==e?e:[]}}),U=Object.prototype.toString
var Y=new p("tag:yaml.org,2002:pairs",{kind:"sequence",resolve:function(e){if(null===e)return!0
var t,n,i,r,o,a=e
for(o=new Array(a.length),t=0,n=a.length;t<n;t+=1){if(i=a[t],"[object Object]"!==U.call(i))return!1
if(1!==(r=Object.keys(i)).length)return!1
o[t]=[r[0],i[r[0]]]}return!0},construct:function(e){if(null===e)return[]
var t,n,i,r,o,a=e
for(o=new Array(a.length),t=0,n=a.length;t<n;t+=1)i=a[t],r=Object.keys(i),o[t]=[r[0],i[r[0]]]
return o}}),P=Object.prototype.hasOwnProperty
var R=new p("tag:yaml.org,2002:set",{kind:"mapping",resolve:function(e){if(null===e)return!0
var t,n=e
for(t in n)if(P.call(n,t)&&null!==n[t])return!1
return!0},construct:function(e){return null!==e?e:{}}}),$=O.extend({implicit:[M,L],explicit:[F,q,Y,R]}),B=Object.prototype.hasOwnProperty,K=1,W=2,H=3,G=4,V=1,Z=2,z=3,J=/[\x00-\x08\x0B\x0C\x0E-\x1F\x7F-\x84\x86-\x9F\uFFFE\uFFFF]|[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?:[^\uD800-\uDBFF]|^)[\uDC00-\uDFFF]/,Q=/[\x85\u2028\u2029]/,X=/[,\[\]\{\}]/,ee=/^(?:!|!!|![a-z\-]+!)$/i,te=/^(?:!|[^,\[\]\{\}])(?:%[0-9a-f]{2}|[0-9a-z\-#;\/\?:@&=\+\$,_\.!~\*'\(\)\[\]])*$/i
function ne(e){return Object.prototype.toString.call(e)}function ie(e){return 10===e||13===e}function re(e){return 9===e||32===e}function oe(e){return 9===e||32===e||10===e||13===e}function ae(e){return 44===e||91===e||93===e||123===e||125===e}function le(e){var t
return 48<=e&&e<=57?e-48:97<=(t=32|e)&&t<=102?t-97+10:-1}function se(e){return 48===e?"\0":97===e?"":98===e?"\b":116===e||9===e?"\t":110===e?"\n":118===e?"\v":102===e?"\f":114===e?"\r":101===e?"":32===e?" ":34===e?'"':47===e?"/":92===e?"\\":78===e?"":95===e?" ":76===e?"\u2028":80===e?"\u2029":""}function ce(e){return e<=65535?String.fromCharCode(e):String.fromCharCode(55296+(e-65536>>10),56320+(e-65536&1023))}for(var ue=new Array(256),pe=new Array(256),fe=0;fe<256;fe++)ue[fe]=se(fe)?1:0,pe[fe]=se(fe)
function de(e,t){this.input=e,this.filename=t.filename||null,this.schema=t.schema||$,this.onWarning=t.onWarning||null,this.legacy=t.legacy||!1,this.json=t.json||!1,this.listener=t.listener||null,this.implicitTypes=this.schema.compiledImplicit,this.typeMap=this.schema.compiledTypeMap,this.length=e.length,this.position=0,this.line=0,this.lineStart=0,this.lineIndent=0,this.firstTabInLine=-1,this.documents=[]}function he(e,t){var n={name:e.filename,buffer:e.input.slice(0,-1),position:e.position,line:e.line,column:e.position-e.lineStart}
return n.snippet=s(n),new o(t,n)}function me(e,t){throw he(e,t)}function ge(e,t){e.onWarning&&e.onWarning.call(null,he(e,t))}var ye={YAML:function(e,t,n){var i,r,o
null!==e.version&&me(e,"duplication of %YAML directive"),1!==n.length&&me(e,"YAML directive accepts exactly one argument"),null===(i=/^([0-9]+)\.([0-9]+)$/.exec(n[0]))&&me(e,"ill-formed argument of the YAML directive"),r=parseInt(i[1],10),o=parseInt(i[2],10),1!==r&&me(e,"unacceptable YAML version of the document"),e.version=n[0],e.checkLineBreaks=o<2,1!==o&&2!==o&&ge(e,"unsupported YAML version of the document")},TAG:function(e,t,n){var i,r
2!==n.length&&me(e,"TAG directive accepts exactly two arguments"),i=n[0],r=n[1],ee.test(i)||me(e,"ill-formed tag handle (first argument) of the TAG directive"),B.call(e.tagMap,i)&&me(e,'there is a previously declared suffix for "'+i+'" tag handle'),te.test(r)||me(e,"ill-formed tag prefix (second argument) of the TAG directive")
try{r=decodeURIComponent(r)}catch(o){me(e,"tag prefix is malformed: "+r)}e.tagMap[i]=r}}
function be(e,t,n,i){var r,o,a,l
if(t<n){if(l=e.input.slice(t,n),i)for(r=0,o=l.length;r<o;r+=1)9===(a=l.charCodeAt(r))||32<=a&&a<=1114111||me(e,"expected valid JSON character")
else J.test(l)&&me(e,"the stream contains non-printable characters")
e.result+=l}}function Ae(e,t,i,r){var o,a,l,s
for(n.isObject(i)||me(e,"cannot merge mappings; the provided source object is unacceptable"),l=0,s=(o=Object.keys(i)).length;l<s;l+=1)a=o[l],B.call(t,a)||(t[a]=i[a],r[a]=!0)}function ve(e,t,n,i,r,o,a,l,s){var c,u
if(Array.isArray(r))for(c=0,u=(r=Array.prototype.slice.call(r)).length;c<u;c+=1)Array.isArray(r[c])&&me(e,"nested arrays are not supported inside keys"),"object"==typeof r&&"[object Object]"===ne(r[c])&&(r[c]="[object Object]")
if("object"==typeof r&&"[object Object]"===ne(r)&&(r="[object Object]"),r=String(r),null===t&&(t={}),"tag:yaml.org,2002:merge"===i)if(Array.isArray(o))for(c=0,u=o.length;c<u;c+=1)Ae(e,t,o[c],n)
else Ae(e,t,o,n)
else e.json||B.call(n,r)||!B.call(t,r)||(e.line=a||e.line,e.lineStart=l||e.lineStart,e.position=s||e.position,me(e,"duplicated mapping key")),"__proto__"===r?Object.defineProperty(t,r,{configurable:!0,enumerable:!0,writable:!0,value:o}):t[r]=o,delete n[r]
return t}function ke(e){var t
10===(t=e.input.charCodeAt(e.position))?e.position++:13===t?(e.position++,10===e.input.charCodeAt(e.position)&&e.position++):me(e,"a line break is expected"),e.line+=1,e.lineStart=e.position,e.firstTabInLine=-1}function we(e,t,n){for(var i=0,r=e.input.charCodeAt(e.position);0!==r;){for(;re(r);)9===r&&-1===e.firstTabInLine&&(e.firstTabInLine=e.position),r=e.input.charCodeAt(++e.position)
if(t&&35===r)do{r=e.input.charCodeAt(++e.position)}while(10!==r&&13!==r&&0!==r)
if(!ie(r))break
for(ke(e),r=e.input.charCodeAt(e.position),i++,e.lineIndent=0;32===r;)e.lineIndent++,r=e.input.charCodeAt(++e.position)}return-1!==n&&0!==i&&e.lineIndent<n&&ge(e,"deficient indentation"),i}function Ce(e){var t,n=e.position
return!(45!==(t=e.input.charCodeAt(n))&&46!==t||t!==e.input.charCodeAt(n+1)||t!==e.input.charCodeAt(n+2)||(n+=3,0!==(t=e.input.charCodeAt(n))&&!oe(t)))}function xe(e,t){1===t?e.result+=" ":t>1&&(e.result+=n.repeat("\n",t-1))}function Ie(e,t){var n,i,r=e.tag,o=e.anchor,a=[],l=!1
if(-1!==e.firstTabInLine)return!1
for(null!==e.anchor&&(e.anchorMap[e.anchor]=a),i=e.input.charCodeAt(e.position);0!==i&&(-1!==e.firstTabInLine&&(e.position=e.firstTabInLine,me(e,"tab characters must not be used in indentation")),45===i)&&oe(e.input.charCodeAt(e.position+1));)if(l=!0,e.position++,we(e,!0,-1)&&e.lineIndent<=t)a.push(null),i=e.input.charCodeAt(e.position)
else if(n=e.line,Oe(e,t,H,!1,!0),a.push(e.result),we(e,!0,-1),i=e.input.charCodeAt(e.position),(e.line===n||e.lineIndent>t)&&0!==i)me(e,"bad indentation of a sequence entry")
else if(e.lineIndent<t)break
return!!l&&(e.tag=r,e.anchor=o,e.kind="sequence",e.result=a,!0)}function Se(e){var t,n,i,r,o=!1,a=!1
if(33!==(r=e.input.charCodeAt(e.position)))return!1
if(null!==e.tag&&me(e,"duplication of a tag property"),60===(r=e.input.charCodeAt(++e.position))?(o=!0,r=e.input.charCodeAt(++e.position)):33===r?(a=!0,n="!!",r=e.input.charCodeAt(++e.position)):n="!",t=e.position,o){do{r=e.input.charCodeAt(++e.position)}while(0!==r&&62!==r)
e.position<e.length?(i=e.input.slice(t,e.position),r=e.input.charCodeAt(++e.position)):me(e,"unexpected end of the stream within a verbatim tag")}else{for(;0!==r&&!oe(r);)33===r&&(a?me(e,"tag suffix cannot contain exclamation marks"):(n=e.input.slice(t-1,e.position+1),ee.test(n)||me(e,"named tag handle cannot contain such characters"),a=!0,t=e.position+1)),r=e.input.charCodeAt(++e.position)
i=e.input.slice(t,e.position),X.test(i)&&me(e,"tag suffix cannot contain flow indicator characters")}i&&!te.test(i)&&me(e,"tag name cannot contain such characters: "+i)
try{i=decodeURIComponent(i)}catch(l){me(e,"tag name is malformed: "+i)}return o?e.tag=i:B.call(e.tagMap,n)?e.tag=e.tagMap[n]+i:"!"===n?e.tag="!"+i:"!!"===n?e.tag="tag:yaml.org,2002:"+i:me(e,'undeclared tag handle "'+n+'"'),!0}function je(e){var t,n
if(38!==(n=e.input.charCodeAt(e.position)))return!1
for(null!==e.anchor&&me(e,"duplication of an anchor property"),n=e.input.charCodeAt(++e.position),t=e.position;0!==n&&!oe(n)&&!ae(n);)n=e.input.charCodeAt(++e.position)
return e.position===t&&me(e,"name of an anchor node must contain at least one character"),e.anchor=e.input.slice(t,e.position),!0}function Oe(e,t,i,r,o){var a,l,s,c,u,p,f,d,h,m=1,g=!1,y=!1
if(null!==e.listener&&e.listener("open",e),e.tag=null,e.anchor=null,e.kind=null,e.result=null,a=l=s=G===i||H===i,r&&we(e,!0,-1)&&(g=!0,e.lineIndent>t?m=1:e.lineIndent===t?m=0:e.lineIndent<t&&(m=-1)),1===m)for(;Se(e)||je(e);)we(e,!0,-1)?(g=!0,s=a,e.lineIndent>t?m=1:e.lineIndent===t?m=0:e.lineIndent<t&&(m=-1)):s=!1
if(s&&(s=g||o),1!==m&&G!==i||(d=K===i||W===i?t:t+1,h=e.position-e.lineStart,1===m?s&&(Ie(e,h)||function(e,t,n){var i,r,o,a,l,s,c,u=e.tag,p=e.anchor,f={},d=Object.create(null),h=null,m=null,g=null,y=!1,b=!1
if(-1!==e.firstTabInLine)return!1
for(null!==e.anchor&&(e.anchorMap[e.anchor]=f),c=e.input.charCodeAt(e.position);0!==c;){if(y||-1===e.firstTabInLine||(e.position=e.firstTabInLine,me(e,"tab characters must not be used in indentation")),i=e.input.charCodeAt(e.position+1),o=e.line,63!==c&&58!==c||!oe(i)){if(a=e.line,l=e.lineStart,s=e.position,!Oe(e,n,W,!1,!0))break
if(e.line===o){for(c=e.input.charCodeAt(e.position);re(c);)c=e.input.charCodeAt(++e.position)
if(58===c)oe(c=e.input.charCodeAt(++e.position))||me(e,"a whitespace character is expected after the key-value separator within a block mapping"),y&&(ve(e,f,d,h,m,null,a,l,s),h=m=g=null),b=!0,y=!1,r=!1,h=e.tag,m=e.result
else{if(!b)return e.tag=u,e.anchor=p,!0
me(e,"can not read an implicit mapping pair; a colon is missed")}}else{if(!b)return e.tag=u,e.anchor=p,!0
me(e,"can not read a block mapping entry; a multiline key may not be an implicit key")}}else 63===c?(y&&(ve(e,f,d,h,m,null,a,l,s),h=m=g=null),b=!0,y=!0,r=!0):y?(y=!1,r=!0):me(e,"incomplete explicit mapping pair; a key node is missed; or followed by a non-tabulated empty line"),e.position+=1,c=i
if((e.line===o||e.lineIndent>t)&&(y&&(a=e.line,l=e.lineStart,s=e.position),Oe(e,t,G,!0,r)&&(y?m=e.result:g=e.result),y||(ve(e,f,d,h,m,g,a,l,s),h=m=g=null),we(e,!0,-1),c=e.input.charCodeAt(e.position)),(e.line===o||e.lineIndent>t)&&0!==c)me(e,"bad indentation of a mapping entry")
else if(e.lineIndent<t)break}return y&&ve(e,f,d,h,m,null,a,l,s),b&&(e.tag=u,e.anchor=p,e.kind="mapping",e.result=f),b}(e,h,d))||function(e,t){var n,i,r,o,a,l,s,c,u,p,f,d,h=!0,m=e.tag,g=e.anchor,y=Object.create(null)
if(91===(d=e.input.charCodeAt(e.position)))a=93,c=!1,o=[]
else{if(123!==d)return!1
a=125,c=!0,o={}}for(null!==e.anchor&&(e.anchorMap[e.anchor]=o),d=e.input.charCodeAt(++e.position);0!==d;){if(we(e,!0,t),(d=e.input.charCodeAt(e.position))===a)return e.position++,e.tag=m,e.anchor=g,e.kind=c?"mapping":"sequence",e.result=o,!0
h?44===d&&me(e,"expected the node content, but found ','"):me(e,"missed comma between flow collection entries"),f=null,l=s=!1,63===d&&oe(e.input.charCodeAt(e.position+1))&&(l=s=!0,e.position++,we(e,!0,t)),n=e.line,i=e.lineStart,r=e.position,Oe(e,t,K,!1,!0),p=e.tag,u=e.result,we(e,!0,t),d=e.input.charCodeAt(e.position),!s&&e.line!==n||58!==d||(l=!0,d=e.input.charCodeAt(++e.position),we(e,!0,t),Oe(e,t,K,!1,!0),f=e.result),c?ve(e,o,y,p,u,f,n,i,r):l?o.push(ve(e,null,y,p,u,f,n,i,r)):o.push(u),we(e,!0,t),44===(d=e.input.charCodeAt(e.position))?(h=!0,d=e.input.charCodeAt(++e.position)):h=!1}me(e,"unexpected end of the stream within a flow collection")}(e,d)?y=!0:(l&&function(e,t){var i,r,o,a,l,s=V,c=!1,u=!1,p=t,f=0,d=!1
if(124===(a=e.input.charCodeAt(e.position)))r=!1
else{if(62!==a)return!1
r=!0}for(e.kind="scalar",e.result="";0!==a;)if(43===(a=e.input.charCodeAt(++e.position))||45===a)V===s?s=43===a?z:Z:me(e,"repeat of a chomping mode identifier")
else{if(!((o=48<=(l=a)&&l<=57?l-48:-1)>=0))break
0===o?me(e,"bad explicit indentation width of a block scalar; it cannot be less than one"):u?me(e,"repeat of an indentation width identifier"):(p=t+o-1,u=!0)}if(re(a)){do{a=e.input.charCodeAt(++e.position)}while(re(a))
if(35===a)do{a=e.input.charCodeAt(++e.position)}while(!ie(a)&&0!==a)}for(;0!==a;){for(ke(e),e.lineIndent=0,a=e.input.charCodeAt(e.position);(!u||e.lineIndent<p)&&32===a;)e.lineIndent++,a=e.input.charCodeAt(++e.position)
if(!u&&e.lineIndent>p&&(p=e.lineIndent),ie(a))f++
else{if(e.lineIndent<p){s===z?e.result+=n.repeat("\n",c?1+f:f):s===V&&c&&(e.result+="\n")
break}for(r?re(a)?(d=!0,e.result+=n.repeat("\n",c?1+f:f)):d?(d=!1,e.result+=n.repeat("\n",f+1)):0===f?c&&(e.result+=" "):e.result+=n.repeat("\n",f):e.result+=n.repeat("\n",c?1+f:f),c=!0,u=!0,f=0,i=e.position;!ie(a)&&0!==a;)a=e.input.charCodeAt(++e.position)
be(e,i,e.position,!1)}}return!0}(e,d)||function(e,t){var n,i,r
if(39!==(n=e.input.charCodeAt(e.position)))return!1
for(e.kind="scalar",e.result="",e.position++,i=r=e.position;0!==(n=e.input.charCodeAt(e.position));)if(39===n){if(be(e,i,e.position,!0),39!==(n=e.input.charCodeAt(++e.position)))return!0
i=e.position,e.position++,r=e.position}else ie(n)?(be(e,i,r,!0),xe(e,we(e,!1,t)),i=r=e.position):e.position===e.lineStart&&Ce(e)?me(e,"unexpected end of the document within a single quoted scalar"):(e.position++,r=e.position)
me(e,"unexpected end of the stream within a single quoted scalar")}(e,d)||function(e,t){var n,i,r,o,a,l,s
if(34!==(l=e.input.charCodeAt(e.position)))return!1
for(e.kind="scalar",e.result="",e.position++,n=i=e.position;0!==(l=e.input.charCodeAt(e.position));){if(34===l)return be(e,n,e.position,!0),e.position++,!0
if(92===l){if(be(e,n,e.position,!0),ie(l=e.input.charCodeAt(++e.position)))we(e,!1,t)
else if(l<256&&ue[l])e.result+=pe[l],e.position++
else if((a=120===(s=l)?2:117===s?4:85===s?8:0)>0){for(r=a,o=0;r>0;r--)(a=le(l=e.input.charCodeAt(++e.position)))>=0?o=(o<<4)+a:me(e,"expected hexadecimal character")
e.result+=ce(o),e.position++}else me(e,"unknown escape sequence")
n=i=e.position}else ie(l)?(be(e,n,i,!0),xe(e,we(e,!1,t)),n=i=e.position):e.position===e.lineStart&&Ce(e)?me(e,"unexpected end of the document within a double quoted scalar"):(e.position++,i=e.position)}me(e,"unexpected end of the stream within a double quoted scalar")}(e,d)?y=!0:!function(e){var t,n,i
if(42!==(i=e.input.charCodeAt(e.position)))return!1
for(i=e.input.charCodeAt(++e.position),t=e.position;0!==i&&!oe(i)&&!ae(i);)i=e.input.charCodeAt(++e.position)
return e.position===t&&me(e,"name of an alias node must contain at least one character"),n=e.input.slice(t,e.position),B.call(e.anchorMap,n)||me(e,'unidentified alias "'+n+'"'),e.result=e.anchorMap[n],we(e,!0,-1),!0}(e)?function(e,t,n){var i,r,o,a,l,s,c,u,p=e.kind,f=e.result
if(oe(u=e.input.charCodeAt(e.position))||ae(u)||35===u||38===u||42===u||33===u||124===u||62===u||39===u||34===u||37===u||64===u||96===u)return!1
if((63===u||45===u)&&(oe(i=e.input.charCodeAt(e.position+1))||n&&ae(i)))return!1
for(e.kind="scalar",e.result="",r=o=e.position,a=!1;0!==u;){if(58===u){if(oe(i=e.input.charCodeAt(e.position+1))||n&&ae(i))break}else if(35===u){if(oe(e.input.charCodeAt(e.position-1)))break}else{if(e.position===e.lineStart&&Ce(e)||n&&ae(u))break
if(ie(u)){if(l=e.line,s=e.lineStart,c=e.lineIndent,we(e,!1,-1),e.lineIndent>=t){a=!0,u=e.input.charCodeAt(e.position)
continue}e.position=o,e.line=l,e.lineStart=s,e.lineIndent=c
break}}a&&(be(e,r,o,!1),xe(e,e.line-l),r=o=e.position,a=!1),re(u)||(o=e.position+1),u=e.input.charCodeAt(++e.position)}return be(e,r,o,!1),!!e.result||(e.kind=p,e.result=f,!1)}(e,d,K===i)&&(y=!0,null===e.tag&&(e.tag="?")):(y=!0,null===e.tag&&null===e.anchor||me(e,"alias node should not have any properties")),null!==e.anchor&&(e.anchorMap[e.anchor]=e.result)):0===m&&(y=s&&Ie(e,h))),null===e.tag)null!==e.anchor&&(e.anchorMap[e.anchor]=e.result)
else if("?"===e.tag){for(null!==e.result&&"scalar"!==e.kind&&me(e,'unacceptable node kind for !<?> tag; it should be "scalar", not "'+e.kind+'"'),c=0,u=e.implicitTypes.length;c<u;c+=1)if((f=e.implicitTypes[c]).resolve(e.result)){e.result=f.construct(e.result),e.tag=f.tag,null!==e.anchor&&(e.anchorMap[e.anchor]=e.result)
break}}else if("!"!==e.tag){if(B.call(e.typeMap[e.kind||"fallback"],e.tag))f=e.typeMap[e.kind||"fallback"][e.tag]
else for(f=null,c=0,u=(p=e.typeMap.multi[e.kind||"fallback"]).length;c<u;c+=1)if(e.tag.slice(0,p[c].tag.length)===p[c].tag){f=p[c]
break}f||me(e,"unknown tag !<"+e.tag+">"),null!==e.result&&f.kind!==e.kind&&me(e,"unacceptable node kind for !<"+e.tag+'> tag; it should be "'+f.kind+'", not "'+e.kind+'"'),f.resolve(e.result,e.tag)?(e.result=f.construct(e.result,e.tag),null!==e.anchor&&(e.anchorMap[e.anchor]=e.result)):me(e,"cannot resolve a node with !<"+e.tag+"> explicit tag")}return null!==e.listener&&e.listener("close",e),null!==e.tag||null!==e.anchor||y}function Te(e){var t,n,i,r,o=e.position,a=!1
for(e.version=null,e.checkLineBreaks=e.legacy,e.tagMap=Object.create(null),e.anchorMap=Object.create(null);0!==(r=e.input.charCodeAt(e.position))&&(we(e,!0,-1),r=e.input.charCodeAt(e.position),!(e.lineIndent>0||37!==r));){for(a=!0,r=e.input.charCodeAt(++e.position),t=e.position;0!==r&&!oe(r);)r=e.input.charCodeAt(++e.position)
for(i=[],(n=e.input.slice(t,e.position)).length<1&&me(e,"directive name must not be less than one character in length");0!==r;){for(;re(r);)r=e.input.charCodeAt(++e.position)
if(35===r){do{r=e.input.charCodeAt(++e.position)}while(0!==r&&!ie(r))
break}if(ie(r))break
for(t=e.position;0!==r&&!oe(r);)r=e.input.charCodeAt(++e.position)
i.push(e.input.slice(t,e.position))}0!==r&&ke(e),B.call(ye,n)?ye[n](e,n,i):ge(e,'unknown document directive "'+n+'"')}we(e,!0,-1),0===e.lineIndent&&45===e.input.charCodeAt(e.position)&&45===e.input.charCodeAt(e.position+1)&&45===e.input.charCodeAt(e.position+2)?(e.position+=3,we(e,!0,-1)):a&&me(e,"directives end mark is expected"),Oe(e,e.lineIndent-1,G,!1,!0),we(e,!0,-1),e.checkLineBreaks&&Q.test(e.input.slice(o,e.position))&&ge(e,"non-ASCII line breaks are interpreted as content"),e.documents.push(e.result),e.position===e.lineStart&&Ce(e)?46===e.input.charCodeAt(e.position)&&(e.position+=3,we(e,!0,-1)):e.position<e.length-1&&me(e,"end of the stream or a document separator is expected")}function Ee(e,t){t=t||{},0!==(e=String(e)).length&&(10!==e.charCodeAt(e.length-1)&&13!==e.charCodeAt(e.length-1)&&(e+="\n"),65279===e.charCodeAt(0)&&(e=e.slice(1)))
var n=new de(e,t),i=e.indexOf("\0")
for(-1!==i&&(n.position=i,me(n,"null byte is not allowed in input")),n.input+="\0";32===n.input.charCodeAt(n.position);)n.lineIndent+=1,n.position+=1
for(;n.position<n.length-1;)Te(n)
return n.documents}var Me={loadAll:function(e,t,n){null!==t&&"object"==typeof t&&void 0===n&&(n=t,t=null)
var i=Ee(e,n)
if("function"!=typeof t)return i
for(var r=0,o=i.length;r<o;r+=1)t(i[r])},load:function(e,t){var n=Ee(e,t)
if(0!==n.length){if(1===n.length)return n[0]
throw new o("expected a single document in the stream, but found more")}}},Le=Object.prototype.toString,Ne=Object.prototype.hasOwnProperty,Fe=65279,_e=9,De=10,qe=13,Ue=32,Ye=33,Pe=34,Re=35,$e=37,Be=38,Ke=39,We=42,He=44,Ge=45,Ve=58,Ze=61,ze=62,Je=63,Qe=64,Xe=91,et=93,tt=96,nt=123,it=124,rt=125,ot={0:"\\0",7:"\\a",8:"\\b",9:"\\t",10:"\\n",11:"\\v",12:"\\f",13:"\\r",27:"\\e",34:'\\"',92:"\\\\",133:"\\N",160:"\\_",8232:"\\L",8233:"\\P"},at=["y","Y","yes","Yes","YES","on","On","ON","n","N","no","No","NO","off","Off","OFF"],lt=/^[-+]?[0-9_]+(?::[0-9_]+)+(?:\.[0-9_]*)?$/
function st(e){var t,i,r
if(t=e.toString(16).toUpperCase(),e<=255)i="x",r=2
else if(e<=65535)i="u",r=4
else{if(!(e<=4294967295))throw new o("code point within a string may not be greater than 0xFFFFFFFF")
i="U",r=8}return"\\"+i+n.repeat("0",r-t.length)+t}var ct=1,ut=2
function pt(e){this.schema=e.schema||$,this.indent=Math.max(1,e.indent||2),this.noArrayIndent=e.noArrayIndent||!1,this.skipInvalid=e.skipInvalid||!1,this.flowLevel=n.isNothing(e.flowLevel)?-1:e.flowLevel,this.styleMap=function(e,t){var n,i,r,o,a,l,s
if(null===t)return{}
for(n={},r=0,o=(i=Object.keys(t)).length;r<o;r+=1)a=i[r],l=String(t[a]),"!!"===a.slice(0,2)&&(a="tag:yaml.org,2002:"+a.slice(2)),(s=e.compiledTypeMap.fallback[a])&&Ne.call(s.styleAliases,l)&&(l=s.styleAliases[l]),n[a]=l
return n}(this.schema,e.styles||null),this.sortKeys=e.sortKeys||!1,this.lineWidth=e.lineWidth||80,this.noRefs=e.noRefs||!1,this.noCompatMode=e.noCompatMode||!1,this.condenseFlow=e.condenseFlow||!1,this.quotingType='"'===e.quotingType?ut:ct,this.forceQuotes=e.forceQuotes||!1,this.replacer="function"==typeof e.replacer?e.replacer:null,this.implicitTypes=this.schema.compiledImplicit,this.explicitTypes=this.schema.compiledExplicit,this.tag=null,this.result="",this.duplicates=[],this.usedDuplicates=null}function ft(e,t){for(var i,r=n.repeat(" ",t),o=0,a=-1,l="",s=e.length;o<s;)-1===(a=e.indexOf("\n",o))?(i=e.slice(o),o=s):(i=e.slice(o,a+1),o=a+1),i.length&&"\n"!==i&&(l+=r),l+=i
return l}function dt(e,t){return"\n"+n.repeat(" ",e.indent*t)}function ht(e){return e===Ue||e===_e}function mt(e){return 32<=e&&e<=126||161<=e&&e<=55295&&8232!==e&&8233!==e||57344<=e&&e<=65533&&e!==Fe||65536<=e&&e<=1114111}function gt(e){return mt(e)&&e!==Fe&&e!==qe&&e!==De}function yt(e,t,n){var i=gt(e),r=i&&!ht(e)
return(n?i:i&&e!==He&&e!==Xe&&e!==et&&e!==nt&&e!==rt)&&e!==Re&&!(t===Ve&&!r)||gt(t)&&!ht(t)&&e===Re||t===Ve&&r}function bt(e,t){var n,i=e.charCodeAt(t)
return i>=55296&&i<=56319&&t+1<e.length&&(n=e.charCodeAt(t+1))>=56320&&n<=57343?1024*(i-55296)+n-56320+65536:i}function At(e){return/^\n* /.test(e)}var vt=1,kt=2,wt=3,Ct=4,xt=5
function It(e,t,n,i,r,o,a,l){var s,c,u=0,p=null,f=!1,d=!1,h=-1!==i,m=-1,g=mt(c=bt(e,0))&&c!==Fe&&!ht(c)&&c!==Ge&&c!==Je&&c!==Ve&&c!==He&&c!==Xe&&c!==et&&c!==nt&&c!==rt&&c!==Re&&c!==Be&&c!==We&&c!==Ye&&c!==it&&c!==Ze&&c!==ze&&c!==Ke&&c!==Pe&&c!==$e&&c!==Qe&&c!==tt&&function(e){return!ht(e)&&e!==Ve}(bt(e,e.length-1))
if(t||a)for(s=0;s<e.length;u>=65536?s+=2:s++){if(!mt(u=bt(e,s)))return xt
g=g&&yt(u,p,l),p=u}else{for(s=0;s<e.length;u>=65536?s+=2:s++){if((u=bt(e,s))===De)f=!0,h&&(d=d||s-m-1>i&&" "!==e[m+1],m=s)
else if(!mt(u))return xt
g=g&&yt(u,p,l),p=u}d=d||h&&s-m-1>i&&" "!==e[m+1]}return f||d?n>9&&At(e)?xt:a?o===ut?xt:kt:d?Ct:wt:!g||a||r(e)?o===ut?xt:kt:vt}function St(e,t,n,i,r){e.dump=function(){if(0===t.length)return e.quotingType===ut?'""':"''"
if(!e.noCompatMode&&(-1!==at.indexOf(t)||lt.test(t)))return e.quotingType===ut?'"'+t+'"':"'"+t+"'"
var a=e.indent*Math.max(1,n),l=-1===e.lineWidth?-1:Math.max(Math.min(e.lineWidth,40),e.lineWidth-a),s=i||e.flowLevel>-1&&n>=e.flowLevel
switch(It(t,s,e.indent,l,(function(t){return function(e,t){var n,i
for(n=0,i=e.implicitTypes.length;n<i;n+=1)if(e.implicitTypes[n].resolve(t))return!0
return!1}(e,t)}),e.quotingType,e.forceQuotes&&!i,r)){case vt:return t
case kt:return"'"+t.replace(/'/g,"''")+"'"
case wt:return"|"+jt(t,e.indent)+Ot(ft(t,a))
case Ct:return">"+jt(t,e.indent)+Ot(ft(function(e,t){var n,i,r=/(\n+)([^\n]*)/g,o=(l=e.indexOf("\n"),l=-1!==l?l:e.length,r.lastIndex=l,Tt(e.slice(0,l),t)),a="\n"===e[0]||" "===e[0]
var l
for(;i=r.exec(e);){var s=i[1],c=i[2]
n=" "===c[0],o+=s+(a||n||""===c?"":"\n")+Tt(c,t),a=n}return o}(t,l),a))
case xt:return'"'+function(e){for(var t,n="",i=0,r=0;r<e.length;i>=65536?r+=2:r++)i=bt(e,r),!(t=ot[i])&&mt(i)?(n+=e[r],i>=65536&&(n+=e[r+1])):n+=t||st(i)
return n}(t)+'"'
default:throw new o("impossible error: invalid scalar style")}}()}function jt(e,t){var n=At(e)?String(t):"",i="\n"===e[e.length-1]
return n+(i&&("\n"===e[e.length-2]||"\n"===e)?"+":i?"":"-")+"\n"}function Ot(e){return"\n"===e[e.length-1]?e.slice(0,-1):e}function Tt(e,t){if(""===e||" "===e[0])return e
for(var n,i,r=/ [^ ]/g,o=0,a=0,l=0,s="";n=r.exec(e);)(l=n.index)-o>t&&(i=a>o?a:l,s+="\n"+e.slice(o,i),o=i+1),a=l
return s+="\n",e.length-o>t&&a>o?s+=e.slice(o,a)+"\n"+e.slice(a+1):s+=e.slice(o),s.slice(1)}function Et(e,t,n,i){var r,o,a,l="",s=e.tag
for(r=0,o=n.length;r<o;r+=1)a=n[r],e.replacer&&(a=e.replacer.call(n,String(r),a)),(Lt(e,t+1,a,!0,!0,!1,!0)||void 0===a&&Lt(e,t+1,null,!0,!0,!1,!0))&&(i&&""===l||(l+=dt(e,t)),e.dump&&De===e.dump.charCodeAt(0)?l+="-":l+="- ",l+=e.dump)
e.tag=s,e.dump=l||"[]"}function Mt(e,t,n){var i,r,a,l,s,c
for(a=0,l=(r=n?e.explicitTypes:e.implicitTypes).length;a<l;a+=1)if(((s=r[a]).instanceOf||s.predicate)&&(!s.instanceOf||"object"==typeof t&&t instanceof s.instanceOf)&&(!s.predicate||s.predicate(t))){if(n?s.multi&&s.representName?e.tag=s.representName(t):e.tag=s.tag:e.tag="?",s.represent){if(c=e.styleMap[s.tag]||s.defaultStyle,"[object Function]"===Le.call(s.represent))i=s.represent(t,c)
else{if(!Ne.call(s.represent,c))throw new o("!<"+s.tag+'> tag resolver accepts not "'+c+'" style')
i=s.represent[c](t,c)}e.dump=i}return!0}return!1}function Lt(e,t,n,i,r,a,l){e.tag=null,e.dump=n,Mt(e,n,!1)||Mt(e,n,!0)
var s,c=Le.call(e.dump),u=i
i&&(i=e.flowLevel<0||e.flowLevel>t)
var p,f,d="[object Object]"===c||"[object Array]"===c
if(d&&(f=-1!==(p=e.duplicates.indexOf(n))),(null!==e.tag&&"?"!==e.tag||f||2!==e.indent&&t>0)&&(r=!1),f&&e.usedDuplicates[p])e.dump="*ref_"+p
else{if(d&&f&&!e.usedDuplicates[p]&&(e.usedDuplicates[p]=!0),"[object Object]"===c)i&&0!==Object.keys(e.dump).length?(function(e,t,n,i){var r,a,l,s,c,u,p="",f=e.tag,d=Object.keys(n)
if(!0===e.sortKeys)d.sort()
else if("function"==typeof e.sortKeys)d.sort(e.sortKeys)
else if(e.sortKeys)throw new o("sortKeys must be a boolean or a function")
for(r=0,a=d.length;r<a;r+=1)u="",i&&""===p||(u+=dt(e,t)),s=n[l=d[r]],e.replacer&&(s=e.replacer.call(n,l,s)),Lt(e,t+1,l,!0,!0,!0)&&((c=null!==e.tag&&"?"!==e.tag||e.dump&&e.dump.length>1024)&&(e.dump&&De===e.dump.charCodeAt(0)?u+="?":u+="? "),u+=e.dump,c&&(u+=dt(e,t)),Lt(e,t+1,s,!0,c)&&(e.dump&&De===e.dump.charCodeAt(0)?u+=":":u+=": ",p+=u+=e.dump))
e.tag=f,e.dump=p||"{}"}(e,t,e.dump,r),f&&(e.dump="&ref_"+p+e.dump)):(function(e,t,n){var i,r,o,a,l,s="",c=e.tag,u=Object.keys(n)
for(i=0,r=u.length;i<r;i+=1)l="",""!==s&&(l+=", "),e.condenseFlow&&(l+='"'),a=n[o=u[i]],e.replacer&&(a=e.replacer.call(n,o,a)),Lt(e,t,o,!1,!1)&&(e.dump.length>1024&&(l+="? "),l+=e.dump+(e.condenseFlow?'"':"")+":"+(e.condenseFlow?"":" "),Lt(e,t,a,!1,!1)&&(s+=l+=e.dump))
e.tag=c,e.dump="{"+s+"}"}(e,t,e.dump),f&&(e.dump="&ref_"+p+" "+e.dump))
else if("[object Array]"===c)i&&0!==e.dump.length?(e.noArrayIndent&&!l&&t>0?Et(e,t-1,e.dump,r):Et(e,t,e.dump,r),f&&(e.dump="&ref_"+p+e.dump)):(function(e,t,n){var i,r,o,a="",l=e.tag
for(i=0,r=n.length;i<r;i+=1)o=n[i],e.replacer&&(o=e.replacer.call(n,String(i),o)),(Lt(e,t,o,!1,!1)||void 0===o&&Lt(e,t,null,!1,!1))&&(""!==a&&(a+=","+(e.condenseFlow?"":" ")),a+=e.dump)
e.tag=l,e.dump="["+a+"]"}(e,t,e.dump),f&&(e.dump="&ref_"+p+" "+e.dump))
else{if("[object String]"!==c){if("[object Undefined]"===c)return!1
if(e.skipInvalid)return!1
throw new o("unacceptable kind of an object to dump "+c)}"?"!==e.tag&&St(e,e.dump,t,a,u)}null!==e.tag&&"?"!==e.tag&&(s=encodeURI("!"===e.tag[0]?e.tag.slice(1):e.tag).replace(/!/g,"%21"),s="!"===e.tag[0]?"!"+s:"tag:yaml.org,2002:"===s.slice(0,18)?"!!"+s.slice(18):"!<"+s+">",e.dump=s+" "+e.dump)}return!0}function Nt(e,t){var n,i,r=[],o=[]
for(Ft(e,r,o),n=0,i=o.length;n<i;n+=1)t.duplicates.push(r[o[n]])
t.usedDuplicates=new Array(i)}function Ft(e,t,n){var i,r,o
if(null!==e&&"object"==typeof e)if(-1!==(r=t.indexOf(e)))-1===n.indexOf(r)&&n.push(r)
else if(t.push(e),Array.isArray(e))for(r=0,o=e.length;r<o;r+=1)Ft(e[r],t,n)
else for(r=0,o=(i=Object.keys(e)).length;r<o;r+=1)Ft(e[i[r]],t,n)}function _t(e,t){return function(){throw new Error("Function yaml."+e+" is removed in js-yaml 4. Use yaml."+t+" instead, which is now safe by default.")}}var Dt=p,qt=h,Ut=b,Yt=j,Pt=O,Rt=$,$t=Me.load,Bt=Me.loadAll,Kt={dump:function(e,t){var n=new pt(t=t||{})
n.noRefs||Nt(e,n)
var i=e
return n.replacer&&(i=n.replacer.call({"":i},"",i)),Lt(n,0,i,!0,!0)?n.dump+"\n":""}}.dump,Wt=o,Ht={binary:F,float:S,map:y,null:A,pairs:Y,set:R,timestamp:M,bool:v,int:C,merge:L,omap:q,seq:g,str:m},Gt=_t("safeLoad","load"),Vt=_t("safeLoadAll","loadAll"),Zt=_t("safeDump","dump"),zt={Type:Dt,Schema:qt,FAILSAFE_SCHEMA:Ut,JSON_SCHEMA:Yt,CORE_SCHEMA:Pt,DEFAULT_SCHEMA:Rt,load:$t,loadAll:Bt,dump:Kt,YAMLException:Wt,types:Ht,safeLoad:Gt,safeLoadAll:Vt,safeDump:Zt}
e.CORE_SCHEMA=Pt,e.DEFAULT_SCHEMA=Rt,e.FAILSAFE_SCHEMA=Ut,e.JSON_SCHEMA=Yt,e.Schema=qt,e.Type=Dt,e.YAMLException=Wt,e.default=zt,e.dump=Kt,e.load=$t,e.loadAll=Bt,e.safeDump=Zt,e.safeLoad=Gt,e.safeLoadAll=Vt,e.types=Ht,Object.defineProperty(e,"__esModule",{value:!0})})),function(e){"object"==typeof exports&&"object"==typeof module?e(require("../../lib/codemirror")):"function"==typeof define&&define.amd?define(["../../lib/codemirror"],e):e(CodeMirror)}((function(e){"use strict"
e.defineMode("yaml",(function(){var e=new RegExp("\\b(("+["true","false","on","off","yes","no"].join(")|(")+"))$","i")
return{token:function(t,n){var i=t.peek(),r=n.escaped
if(n.escaped=!1,"#"==i&&(0==t.pos||/\s/.test(t.string.charAt(t.pos-1))))return t.skipToEnd(),"comment"
if(t.match(/^('([^']|\\.)*'?|"([^"]|\\.)*"?)/))return"string"
if(n.literal&&t.indentation()>n.keyCol)return t.skipToEnd(),"string"
if(n.literal&&(n.literal=!1),t.sol()){if(n.keyCol=0,n.pair=!1,n.pairStart=!1,t.match(/---/))return"def"
if(t.match(/\.\.\./))return"def"
if(t.match(/\s*-\s+/))return"meta"}if(t.match(/^(\{|\}|\[|\])/))return"{"==i?n.inlinePairs++:"}"==i?n.inlinePairs--:"["==i?n.inlineList++:n.inlineList--,"meta"
if(n.inlineList>0&&!r&&","==i)return t.next(),"meta"
if(n.inlinePairs>0&&!r&&","==i)return n.keyCol=0,n.pair=!1,n.pairStart=!1,t.next(),"meta"
if(n.pairStart){if(t.match(/^\s*(\||\>)\s*/))return n.literal=!0,"meta"
if(t.match(/^\s*(\&|\*)[a-z0-9\._-]+\b/i))return"variable-2"
if(0==n.inlinePairs&&t.match(/^\s*-?[0-9\.\,]+\s?$/))return"number"
if(n.inlinePairs>0&&t.match(/^\s*-?[0-9\.\,]+\s?(?=(,|}))/))return"number"
if(t.match(e))return"keyword"}return!n.pair&&t.match(/^\s*(?:[,\[\]{}&*!|>'"%@`][^\s'":]|[^,\[\]{}#&*!|>'"%@`])[^#]*?(?=\s*:($|\s))/)?(n.pair=!0,n.keyCol=t.indentation(),"atom"):n.pair&&t.match(/^:\s*/)?(n.pairStart=!0,"meta"):(n.pairStart=!1,n.escaped="\\"==i,t.next(),null)},startState:function(){return{pair:!1,pairStart:!1,keyCol:0,inlinePairs:0,inlineList:0,literal:!1,escaped:!1}}}})),e.defineMIME("text/x-yaml","yaml")}))
