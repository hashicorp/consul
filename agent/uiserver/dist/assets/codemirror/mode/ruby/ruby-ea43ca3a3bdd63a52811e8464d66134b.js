(function(e){"object"==typeof exports&&"object"==typeof module?e(require("../../lib/codemirror")):"function"==typeof define&&define.amd?define(["../../lib/codemirror"],e):e(CodeMirror)})((function(e){"use strict"
e.defineMode("ruby",(function(e){function t(e){for(var t={},n=0,r=e.length;n<r;++n)t[e[n]]=!0
return t}var n,r=t(["alias","and","BEGIN","begin","break","case","class","def","defined?","do","else","elsif","END","end","ensure","false","for","if","in","module","next","not","or","redo","rescue","retry","return","self","super","then","true","undef","unless","until","when","while","yield","nil","raise","throw","catch","fail","loop","callcc","caller","lambda","proc","public","protected","private","require","load","require_relative","extend","autoload","__END__","__FILE__","__LINE__","__dir__"]),i=t(["def","class","case","for","while","until","module","then","catch","loop","proc","begin"]),o=t(["end","until"]),a={"[":"]","{":"}","(":")"}
function u(e,t,n){return n.tokenize.push(e),e(t,n)}function f(e,t){if(e.sol()&&e.match("=begin")&&e.eol())return t.tokenize.push(s),"comment"
if(e.eatSpace())return null
var r,i,o=e.next()
if("`"==o||"'"==o||'"'==o)return u(c(o,"string",'"'==o||"`"==o),e,t)
if("/"==o){var f=e.current().length
if(e.skipTo("/")){var l=e.current().length
e.backUp(e.current().length-f)
for(var d=0;e.current().length<l;){var p=e.next()
if("("==p?d+=1:")"==p&&(d-=1),d<0)break}if(e.backUp(e.current().length-f),0==d)return u(c(o,"string-2",!0),e,t)}return"operator"}if("%"==o){var k="string",h=!0
e.eat("s")?k="atom":e.eat(/[WQ]/)?k="string":e.eat(/[r]/)?k="string-2":e.eat(/[wxq]/)&&(k="string",h=!1)
var m=e.eat(/[^\w\s=]/)
return m?(a.propertyIsEnumerable(m)&&(m=a[m]),u(c(m,k,h,!0),e,t)):"operator"}if("#"==o)return e.skipToEnd(),"comment"
if("<"==o&&(r=e.match(/^<-?[\`\"\']?([a-zA-Z_?]\w*)[\`\"\']?(?:;|$)/)))return u((i=r[1],function(e,t){return e.match(i)?t.tokenize.pop():e.skipToEnd(),"string"}),e,t)
if("0"==o)return e.eat("x")?e.eatWhile(/[\da-fA-F]/):e.eat("b")?e.eatWhile(/[01]/):e.eatWhile(/[0-7]/),"number"
if(/\d/.test(o))return e.match(/^[\d_]*(?:\.[\d_]+)?(?:[eE][+\-]?[\d_]+)?/),"number"
if("?"==o){for(;e.match(/^\\[CM]-/););return e.eat("\\")?e.eatWhile(/\w/):e.next(),"string"}if(":"==o)return e.eat("'")?u(c("'","atom",!1),e,t):e.eat('"')?u(c('"',"atom",!0),e,t):e.eat(/[\<\>]/)?(e.eat(/[\<\>]/),"atom"):e.eat(/[\+\-\*\/\&\|\:\!]/)?"atom":e.eat(/[a-zA-Z$@_\xa1-\uffff]/)?(e.eatWhile(/[\w$\xa1-\uffff]/),e.eat(/[\?\!\=]/),"atom"):"operator"
if("@"==o&&e.match(/^@?[a-zA-Z_\xa1-\uffff]/))return e.eat("@"),e.eatWhile(/[\w\xa1-\uffff]/),"variable-2"
if("$"==o)return e.eat(/[a-zA-Z_]/)?e.eatWhile(/[\w]/):e.eat(/\d/)?e.eat(/\d/):e.next(),"variable-3"
if(/[a-zA-Z_\xa1-\uffff]/.test(o))return e.eatWhile(/[\w\xa1-\uffff]/),e.eat(/[\?\!]/),e.eat(":")?"atom":"ident"
if("|"!=o||!t.varList&&"{"!=t.lastTok&&"do"!=t.lastTok){if(/[\(\)\[\]{}\\;]/.test(o))return n=o,null
if("-"==o&&e.eat(">"))return"arrow"
if(/[=+\-\/*:\.^%<>~|]/.test(o)){var x=e.eatWhile(/[=+\-\/*:\.^%<>~|]/)
return"."!=o||x||(n="."),"operator"}return null}return n="|",null}function l(e){return e||(e=1),function(t,n){if("}"==t.peek()){if(1==e)return n.tokenize.pop(),n.tokenize[n.tokenize.length-1](t,n)
n.tokenize[n.tokenize.length-1]=l(e-1)}else"{"==t.peek()&&(n.tokenize[n.tokenize.length-1]=l(e+1))
return f(t,n)}}function d(){var e=!1
return function(t,n){return e?(n.tokenize.pop(),n.tokenize[n.tokenize.length-1](t,n)):(e=!0,f(t,n))}}function c(e,t,n,r){return function(i,o){var a,u=!1
for("read-quoted-paused"===o.context.type&&(o.context=o.context.prev,i.eat("}"));null!=(a=i.next());){if(a==e&&(r||!u)){o.tokenize.pop()
break}if(n&&"#"==a&&!u){if(i.eat("{")){"}"==e&&(o.context={prev:o.context,type:"read-quoted-paused"}),o.tokenize.push(l())
break}if(/[@\$]/.test(i.peek())){o.tokenize.push(d())
break}}u=!u&&"\\"==a}return t}}function s(e,t){return e.sol()&&e.match("=end")&&e.eol()&&t.tokenize.pop(),e.skipToEnd(),"comment"}return{startState:function(){return{tokenize:[f],indented:0,context:{type:"top",indented:-e.indentUnit},continuedLine:!1,lastTok:null,varList:!1}},token:function(e,t){n=null,e.sol()&&(t.indented=e.indentation())
var a,u=t.tokenize[t.tokenize.length-1](e,t),f=n
if("ident"==u){var l=e.current()
"keyword"==(u="."==t.lastTok?"property":r.propertyIsEnumerable(e.current())?"keyword":/^[A-Z]/.test(l)?"tag":"def"==t.lastTok||"class"==t.lastTok||t.varList?"def":"variable")&&(f=l,i.propertyIsEnumerable(l)?a="indent":o.propertyIsEnumerable(l)?a="dedent":"if"!=l&&"unless"!=l||e.column()!=e.indentation()?"do"==l&&t.context.indented<t.indented&&(a="indent"):a="indent")}return(n||u&&"comment"!=u)&&(t.lastTok=f),"|"==n&&(t.varList=!t.varList),"indent"==a||/[\(\[\{]/.test(n)?t.context={prev:t.context,type:n||u,indented:t.indented}:("dedent"==a||/[\)\]\}]/.test(n))&&t.context.prev&&(t.context=t.context.prev),e.eol()&&(t.continuedLine="\\"==n||"operator"==u),u},indent:function(t,n){if(t.tokenize[t.tokenize.length-1]!=f)return 0
var r=n&&n.charAt(0),i=t.context,o=i.type==a[r]||"keyword"==i.type&&/^(?:end|until|else|elsif|when|rescue)\b/.test(n)
return i.indented+(o?0:e.indentUnit)+(t.continuedLine?e.indentUnit:0)},electricInput:/^\s*(?:end|rescue|\})$/,lineComment:"#"}})),e.defineMIME("text/x-ruby","ruby")}))
