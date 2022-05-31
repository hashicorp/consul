(function(t){"object"==typeof exports&&"object"==typeof module?t(require("../../lib/codemirror")):"function"==typeof define&&define.amd?define(["../../lib/codemirror"],t):t(CodeMirror)})((function(t){"use strict"
var e={autoSelfClosers:{area:!0,base:!0,br:!0,col:!0,command:!0,embed:!0,frame:!0,hr:!0,img:!0,input:!0,keygen:!0,link:!0,meta:!0,param:!0,source:!0,track:!0,wbr:!0,menuitem:!0},implicitlyClosed:{dd:!0,li:!0,optgroup:!0,option:!0,p:!0,rp:!0,rt:!0,tbody:!0,td:!0,tfoot:!0,th:!0,tr:!0},contextGrabbers:{dd:{dd:!0,dt:!0},dt:{dd:!0,dt:!0},li:{li:!0},option:{option:!0,optgroup:!0},optgroup:{optgroup:!0},p:{address:!0,article:!0,aside:!0,blockquote:!0,dir:!0,div:!0,dl:!0,fieldset:!0,footer:!0,form:!0,h1:!0,h2:!0,h3:!0,h4:!0,h5:!0,h6:!0,header:!0,hgroup:!0,hr:!0,menu:!0,nav:!0,ol:!0,p:!0,pre:!0,section:!0,table:!0,ul:!0},rp:{rp:!0,rt:!0},rt:{rp:!0,rt:!0},tbody:{tbody:!0,tfoot:!0},td:{td:!0,th:!0},tfoot:{tbody:!0},th:{td:!0,th:!0},thead:{tbody:!0,tfoot:!0},tr:{tr:!0}},doNotIndent:{pre:!0},allowUnquoted:!0,allowMissing:!0,caseFold:!0},n={autoSelfClosers:{},implicitlyClosed:{},contextGrabbers:{},doNotIndent:{},allowUnquoted:!1,allowMissing:!1,caseFold:!1}
t.defineMode("xml",(function(r,o){var a,i,l=r.indentUnit,u={},d=o.htmlMode?e:n
for(var c in d)u[c]=d[c]
for(var c in o)u[c]=o[c]
function s(t,e){function n(n){return e.tokenize=n,n(t,e)}var r=t.next()
return"<"==r?t.eat("!")?t.eat("[")?t.match("CDATA[")?n(m("atom","]]>")):null:t.match("--")?n(m("comment","--\x3e")):t.match("DOCTYPE",!0,!0)?(t.eatWhile(/[\w\._\-]/),n(function t(e){return function(n,r){for(var o;null!=(o=n.next());){if("<"==o)return r.tokenize=t(e+1),r.tokenize(n,r)
if(">"==o){if(1==e){r.tokenize=s
break}return r.tokenize=t(e-1),r.tokenize(n,r)}}return"meta"}}(1))):null:t.eat("?")?(t.eatWhile(/[\w\._\-]/),e.tokenize=m("meta","?>"),"meta"):(a=t.eat("/")?"closeTag":"openTag",e.tokenize=f,"tag bracket"):"&"==r?(t.eat("#")?t.eat("x")?t.eatWhile(/[a-fA-F\d]/)&&t.eat(";"):t.eatWhile(/[\d]/)&&t.eat(";"):t.eatWhile(/[\w\.\-:]/)&&t.eat(";"))?"atom":"error":(t.eatWhile(/[^&<]/),null)}function f(t,e){var n,r,o=t.next()
if(">"==o||"/"==o&&t.eat(">"))return e.tokenize=s,a=">"==o?"endTag":"selfcloseTag","tag bracket"
if("="==o)return a="equals",null
if("<"==o){e.tokenize=s,e.state=x,e.tagName=e.tagStart=null
var i=e.tokenize(t,e)
return i?i+" tag error":"tag error"}return/[\'\"]/.test(o)?(e.tokenize=(n=o,(r=function(t,e){for(;!t.eol();)if(t.next()==n){e.tokenize=f
break}return"string"}).isInAttribute=!0,r),e.stringStartCol=t.column(),e.tokenize(t,e)):(t.match(/^[^\s\u00a0=<>\"\']*[^\s\u00a0=<>\"\'\/]/),"word")}function m(t,e){return function(n,r){for(;!n.eol();){if(n.match(e)){r.tokenize=s
break}n.next()}return t}}function g(t,e,n){this.prev=t.context,this.tagName=e,this.indent=t.indented,this.startOfLine=n,(u.doNotIndent.hasOwnProperty(e)||t.context&&t.context.noIndent)&&(this.noIndent=!0)}function p(t){t.context&&(t.context=t.context.prev)}function h(t,e){for(var n;;){if(!t.context)return
if(n=t.context.tagName,!u.contextGrabbers.hasOwnProperty(n)||!u.contextGrabbers[n].hasOwnProperty(e))return
p(t)}}function x(t,e,n){return"openTag"==t?(n.tagStart=e.column(),b):"closeTag"==t?k:x}function b(t,e,n){return"word"==t?(n.tagName=e.current(),i="tag",y):(i="error",b)}function k(t,e,n){if("word"==t){var r=e.current()
return n.context&&n.context.tagName!=r&&u.implicitlyClosed.hasOwnProperty(n.context.tagName)&&p(n),n.context&&n.context.tagName==r||!1===u.matchClosing?(i="tag",w):(i="tag error",v)}return i="error",v}function w(t,e,n){return"endTag"!=t?(i="error",w):(p(n),x)}function v(t,e,n){return i="error",w(t,0,n)}function y(t,e,n){if("word"==t)return i="attribute",z
if("endTag"==t||"selfcloseTag"==t){var r=n.tagName,o=n.tagStart
return n.tagName=n.tagStart=null,"selfcloseTag"==t||u.autoSelfClosers.hasOwnProperty(r)?h(n,r):(h(n,r),n.context=new g(n,r,o==n.indented)),x}return i="error",y}function z(t,e,n){return"equals"==t?N:(u.allowMissing||(i="error"),y(t,0,n))}function N(t,e,n){return"string"==t?T:"word"==t&&u.allowUnquoted?(i="string",y):(i="error",y(t,0,n))}function T(t,e,n){return"string"==t?T:y(t,0,n)}return s.isInText=!0,{startState:function(t){var e={tokenize:s,state:x,indented:t||0,tagName:null,tagStart:null,context:null}
return null!=t&&(e.baseIndent=t),e},token:function(t,e){if(!e.tagName&&t.sol()&&(e.indented=t.indentation()),t.eatSpace())return null
a=null
var n=e.tokenize(t,e)
return(n||a)&&"comment"!=n&&(i=null,e.state=e.state(a||n,t,e),i&&(n="error"==i?n+" error":i)),n},indent:function(e,n,r){var o=e.context
if(e.tokenize.isInAttribute)return e.tagStart==e.indented?e.stringStartCol+1:e.indented+l
if(o&&o.noIndent)return t.Pass
if(e.tokenize!=f&&e.tokenize!=s)return r?r.match(/^(\s*)/)[0].length:0
if(e.tagName)return!1!==u.multilineTagIndentPastTag?e.tagStart+e.tagName.length+2:e.tagStart+l*(u.multilineTagIndentFactor||1)
if(u.alignCDATA&&/<!\[CDATA\[/.test(n))return 0
var a=n&&/^<(\/)?([\w_:\.-]*)/.exec(n)
if(a&&a[1])for(;o;){if(o.tagName==a[2]){o=o.prev
break}if(!u.implicitlyClosed.hasOwnProperty(o.tagName))break
o=o.prev}else if(a)for(;o;){var i=u.contextGrabbers[o.tagName]
if(!i||!i.hasOwnProperty(a[2]))break
o=o.prev}for(;o&&o.prev&&!o.startOfLine;)o=o.prev
return o?o.indent+l:e.baseIndent||0},electricInput:/<\/[\s\w:]+>$/,blockCommentStart:"\x3c!--",blockCommentEnd:"--\x3e",configuration:u.htmlMode?"html":"xml",helperType:u.htmlMode?"html":"xml",skipAttribute:function(t){t.state==N&&(t.state=y)}}})),t.defineMIME("text/xml","xml"),t.defineMIME("application/xml","xml"),t.mimeModes.hasOwnProperty("text/html")||t.defineMIME("text/html",{name:"xml",htmlMode:!0})}))
