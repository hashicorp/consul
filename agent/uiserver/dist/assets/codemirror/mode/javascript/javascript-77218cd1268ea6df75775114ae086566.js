var jsonlint=function(){var e={trace:function(){},yy:{},symbols_:{error:2,JSONString:3,STRING:4,JSONNumber:5,NUMBER:6,JSONNullLiteral:7,NULL:8,JSONBooleanLiteral:9,TRUE:10,FALSE:11,JSONText:12,JSONValue:13,EOF:14,JSONObject:15,JSONArray:16,"{":17,"}":18,JSONMemberList:19,JSONMember:20,":":21,",":22,"[":23,"]":24,JSONElementList:25,$accept:0,$end:1},terminals_:{2:"error",4:"STRING",6:"NUMBER",8:"NULL",10:"TRUE",11:"FALSE",14:"EOF",17:"{",18:"}",21:":",22:",",23:"[",24:"]"},productions_:[0,[3,1],[5,1],[7,1],[9,1],[9,1],[12,2],[13,1],[13,1],[13,1],[13,1],[13,1],[13,1],[15,2],[15,3],[20,3],[19,1],[19,3],[16,2],[16,3],[25,1],[25,3]],performAction:function(e,t,r,n,i,a){var s=a.length-1
switch(i){case 1:this.$=e.replace(/\\(\\|")/g,"$1").replace(/\\n/g,"\n").replace(/\\r/g,"\r").replace(/\\t/g,"\t").replace(/\\v/g,"\v").replace(/\\f/g,"\f").replace(/\\b/g,"\b")
break
case 2:this.$=Number(e)
break
case 3:this.$=null
break
case 4:this.$=!0
break
case 5:this.$=!1
break
case 6:return this.$=a[s-1]
case 13:this.$={}
break
case 14:this.$=a[s-1]
break
case 15:this.$=[a[s-2],a[s]]
break
case 16:this.$={},this.$[a[s][0]]=a[s][1]
break
case 17:this.$=a[s-2],a[s-2][a[s][0]]=a[s][1]
break
case 18:this.$=[]
break
case 19:this.$=a[s-1]
break
case 20:this.$=[a[s]]
break
case 21:this.$=a[s-2],a[s-2].push(a[s])}},table:[{3:5,4:[1,12],5:6,6:[1,13],7:3,8:[1,9],9:4,10:[1,10],11:[1,11],12:1,13:2,15:7,16:8,17:[1,14],23:[1,15]},{1:[3]},{14:[1,16]},{14:[2,7],18:[2,7],22:[2,7],24:[2,7]},{14:[2,8],18:[2,8],22:[2,8],24:[2,8]},{14:[2,9],18:[2,9],22:[2,9],24:[2,9]},{14:[2,10],18:[2,10],22:[2,10],24:[2,10]},{14:[2,11],18:[2,11],22:[2,11],24:[2,11]},{14:[2,12],18:[2,12],22:[2,12],24:[2,12]},{14:[2,3],18:[2,3],22:[2,3],24:[2,3]},{14:[2,4],18:[2,4],22:[2,4],24:[2,4]},{14:[2,5],18:[2,5],22:[2,5],24:[2,5]},{14:[2,1],18:[2,1],21:[2,1],22:[2,1],24:[2,1]},{14:[2,2],18:[2,2],22:[2,2],24:[2,2]},{3:20,4:[1,12],18:[1,17],19:18,20:19},{3:5,4:[1,12],5:6,6:[1,13],7:3,8:[1,9],9:4,10:[1,10],11:[1,11],13:23,15:7,16:8,17:[1,14],23:[1,15],24:[1,21],25:22},{1:[2,6]},{14:[2,13],18:[2,13],22:[2,13],24:[2,13]},{18:[1,24],22:[1,25]},{18:[2,16],22:[2,16]},{21:[1,26]},{14:[2,18],18:[2,18],22:[2,18],24:[2,18]},{22:[1,28],24:[1,27]},{22:[2,20],24:[2,20]},{14:[2,14],18:[2,14],22:[2,14],24:[2,14]},{3:20,4:[1,12],20:29},{3:5,4:[1,12],5:6,6:[1,13],7:3,8:[1,9],9:4,10:[1,10],11:[1,11],13:30,15:7,16:8,17:[1,14],23:[1,15]},{14:[2,19],18:[2,19],22:[2,19],24:[2,19]},{3:5,4:[1,12],5:6,6:[1,13],7:3,8:[1,9],9:4,10:[1,10],11:[1,11],13:31,15:7,16:8,17:[1,14],23:[1,15]},{18:[2,17],22:[2,17]},{18:[2,15],22:[2,15]},{22:[2,21],24:[2,21]}],defaultActions:{16:[2,6]},parseError:function(e){throw new Error(e)},parse:function(e){var t=this,r=[0],n=[null],i=[],a=this.table,s="",o=0,l=0,c=0
this.lexer.setInput(e),this.lexer.yy=this.yy,this.yy.lexer=this.lexer,void 0===this.lexer.yylloc&&(this.lexer.yylloc={})
var u=this.lexer.yylloc
function f(){var e
return"number"!=typeof(e=t.lexer.lex()||1)&&(e=t.symbols_[e]||e),e}i.push(u),"function"==typeof this.yy.parseError&&(this.parseError=this.yy.parseError)
for(var h,p,d,y,m,v,g,b,x,k,w={};;){if(d=r[r.length-1],this.defaultActions[d]?y=this.defaultActions[d]:(null==h&&(h=f()),y=a[d]&&a[d][h]),void 0===y||!y.length||!y[0]){if(!c){for(v in x=[],a[d])this.terminals_[v]&&v>2&&x.push("'"+this.terminals_[v]+"'")
var _=""
_=this.lexer.showPosition?"Parse error on line "+(o+1)+":\n"+this.lexer.showPosition()+"\nExpecting "+x.join(", ")+", got '"+this.terminals_[h]+"'":"Parse error on line "+(o+1)+": Unexpected "+(1==h?"end of input":"'"+(this.terminals_[h]||h)+"'"),this.parseError(_,{text:this.lexer.match,token:this.terminals_[h]||h,line:this.lexer.yylineno,loc:u,expected:x})}if(3==c){if(1==h)throw new Error(_||"Parsing halted.")
l=this.lexer.yyleng,s=this.lexer.yytext,o=this.lexer.yylineno,u=this.lexer.yylloc,h=f()}for(;!(2..toString()in a[d]);){if(0==d)throw new Error(_||"Parsing halted.")
k=1,r.length=r.length-2*k,n.length=n.length-k,i.length=i.length-k,d=r[r.length-1]}p=h,h=2,y=a[d=r[r.length-1]]&&a[d][2],c=3}if(y[0]instanceof Array&&y.length>1)throw new Error("Parse Error: multiple actions possible at state: "+d+", token: "+h)
switch(y[0]){case 1:r.push(h),n.push(this.lexer.yytext),i.push(this.lexer.yylloc),r.push(y[1]),h=null,p?(h=p,p=null):(l=this.lexer.yyleng,s=this.lexer.yytext,o=this.lexer.yylineno,u=this.lexer.yylloc,c>0&&c--)
break
case 2:if(g=this.productions_[y[1]][1],w.$=n[n.length-g],w._$={first_line:i[i.length-(g||1)].first_line,last_line:i[i.length-1].last_line,first_column:i[i.length-(g||1)].first_column,last_column:i[i.length-1].last_column},void 0!==(m=this.performAction.call(w,s,l,o,this.yy,y[1],n,i)))return m
g&&(r=r.slice(0,-1*g*2),n=n.slice(0,-1*g),i=i.slice(0,-1*g)),r.push(this.productions_[y[1]][0]),n.push(w.$),i.push(w._$),b=a[r[r.length-2]][r[r.length-1]],r.push(b)
break
case 3:return!0}}return!0}},t=function(){var e={EOF:1,parseError:function(e,t){if(!this.yy.parseError)throw new Error(e)
this.yy.parseError(e,t)},setInput:function(e){return this._input=e,this._more=this._less=this.done=!1,this.yylineno=this.yyleng=0,this.yytext=this.matched=this.match="",this.conditionStack=["INITIAL"],this.yylloc={first_line:1,first_column:0,last_line:1,last_column:0},this},input:function(){var e=this._input[0]
return this.yytext+=e,this.yyleng++,this.match+=e,this.matched+=e,e.match(/\n/)&&this.yylineno++,this._input=this._input.slice(1),e},unput:function(e){return this._input=e+this._input,this},more:function(){return this._more=!0,this},less:function(e){this._input=this.match.slice(e)+this._input},pastInput:function(){var e=this.matched.substr(0,this.matched.length-this.match.length)
return(e.length>20?"...":"")+e.substr(-20).replace(/\n/g,"")},upcomingInput:function(){var e=this.match
return e.length<20&&(e+=this._input.substr(0,20-e.length)),(e.substr(0,20)+(e.length>20?"...":"")).replace(/\n/g,"")},showPosition:function(){var e=this.pastInput(),t=new Array(e.length+1).join("-")
return e+this.upcomingInput()+"\n"+t+"^"},next:function(){if(this.done)return this.EOF
var e,t,r,n,i
this._input||(this.done=!0),this._more||(this.yytext="",this.match="")
for(var a=this._currentRules(),s=0;s<a.length&&(!(r=this._input.match(this.rules[a[s]]))||t&&!(r[0].length>t[0].length)||(t=r,n=s,this.options.flex));s++);return t?((i=t[0].match(/\n.*/g))&&(this.yylineno+=i.length),this.yylloc={first_line:this.yylloc.last_line,last_line:this.yylineno+1,first_column:this.yylloc.last_column,last_column:i?i[i.length-1].length-1:this.yylloc.last_column+t[0].length},this.yytext+=t[0],this.match+=t[0],this.yyleng=this.yytext.length,this._more=!1,this._input=this._input.slice(t[0].length),this.matched+=t[0],e=this.performAction.call(this,this.yy,this,a[n],this.conditionStack[this.conditionStack.length-1]),this.done&&this._input&&(this.done=!1),e||void 0):""===this._input?this.EOF:void this.parseError("Lexical error on line "+(this.yylineno+1)+". Unrecognized text.\n"+this.showPosition(),{text:"",token:null,line:this.yylineno})},lex:function(){var e=this.next()
return void 0!==e?e:this.lex()},begin:function(e){this.conditionStack.push(e)},popState:function(){return this.conditionStack.pop()},_currentRules:function(){return this.conditions[this.conditionStack[this.conditionStack.length-1]].rules},topState:function(){return this.conditionStack[this.conditionStack.length-2]},pushState:function(e){this.begin(e)},options:{},performAction:function(e,t,r){switch(r){case 0:break
case 1:return 6
case 2:return t.yytext=t.yytext.substr(1,t.yyleng-2),4
case 3:return 17
case 4:return 18
case 5:return 23
case 6:return 24
case 7:return 22
case 8:return 21
case 9:return 10
case 10:return 11
case 11:return 8
case 12:return 14
case 13:return"INVALID"}},rules:[/^(?:\s+)/,/^(?:(-?([0-9]|[1-9][0-9]+))(\.[0-9]+)?([eE][-+]?[0-9]+)?\b)/,/^(?:"(?:\\[\\"bfnrt/]|\\u[a-fA-F0-9]{4}|[^\\\0-\x09\x0a-\x1f"])*")/,/^(?:\{)/,/^(?:\})/,/^(?:\[)/,/^(?:\])/,/^(?:,)/,/^(?::)/,/^(?:true\b)/,/^(?:false\b)/,/^(?:null\b)/,/^(?:$)/,/^(?:.)/],conditions:{INITIAL:{rules:[0,1,2,3,4,5,6,7,8,9,10,11,12,13],inclusive:!0}}}
return e}()
return e.lexer=t,e}()
"undefined"!=typeof require&&"undefined"!=typeof exports&&(exports.parser=jsonlint,exports.parse=function(){return jsonlint.parse.apply(jsonlint,arguments)},exports.main=function(e){if(!e[1])throw new Error("Usage: "+e[0]+" FILE")
if("undefined"!=typeof process)var t=require("fs").readFileSync(require("path").join(process.cwd(),e[1]),"utf8")
else t=require("file").path(require("file").cwd()).join(e[1]).read({charset:"utf-8"})
return exports.parser.parse(t)},"undefined"!=typeof module&&require.main===module&&exports.main("undefined"!=typeof process?process.argv.slice(1):require("system").args)),function(e){"object"==typeof exports&&"object"==typeof module?e(require("../../lib/codemirror")):"function"==typeof define&&define.amd?define(["../../lib/codemirror"],e):e(CodeMirror)}((function(e){"use strict"
function t(e,t,r){return/^(?:operator|sof|keyword c|case|new|[\[{}\(,;:]|=>)$/.test(t.lastType)||"quasi"==t.lastType&&/\{\s*$/.test(e.string.slice(0,e.pos-(r||0)))}e.defineMode("javascript",(function(r,n){var i,a,s=r.indentUnit,o=n.statementIndent,l=n.jsonld,c=n.json||l,u=n.typescript,f=n.wordCharacters||/[\w$\xa1-\uffff]/,h=function(){function e(e){return{type:e,style:"keyword"}}var t=e("keyword a"),r=e("keyword b"),n=e("keyword c"),i=e("operator"),a={type:"atom",style:"atom"},s={if:e("if"),while:t,with:t,else:r,do:r,try:r,finally:r,return:n,break:n,continue:n,new:e("new"),delete:n,throw:n,debugger:n,var:e("var"),const:e("var"),let:e("var"),function:e("function"),catch:e("catch"),for:e("for"),switch:e("switch"),case:e("case"),default:e("default"),in:i,typeof:i,instanceof:i,true:a,false:a,null:a,undefined:a,NaN:a,Infinity:a,this:e("this"),class:e("class"),super:e("atom"),yield:n,export:e("export"),import:e("import"),extends:n,await:n,async:e("async")}
if(u){var o={type:"variable",style:"variable-3"},l={interface:e("class"),implements:n,namespace:n,module:e("module"),enum:e("module"),public:e("modifier"),private:e("modifier"),protected:e("modifier"),abstract:e("modifier"),as:i,string:o,number:o,boolean:o,any:o}
for(var c in l)s[c]=l[c]}return s}(),p=/[+\-*&%=<>!?|~^]/,d=/^@(context|id|value|language|type|container|list|set|reverse|index|base|vocab|graph)"/
function y(e,t,r){return i=e,a=r,t}function m(e,r){var n,i=e.next()
if('"'==i||"'"==i)return r.tokenize=(n=i,function(e,t){var r,i=!1
if(l&&"@"==e.peek()&&e.match(d))return t.tokenize=m,y("jsonld-keyword","meta")
for(;null!=(r=e.next())&&(r!=n||i);)i=!i&&"\\"==r
return i||(t.tokenize=m),y("string","string")}),r.tokenize(e,r)
if("."==i&&e.match(/^\d+(?:[eE][+\-]?\d+)?/))return y("number","number")
if("."==i&&e.match(".."))return y("spread","meta")
if(/[\[\]{}\(\),;\:\.]/.test(i))return y(i)
if("="==i&&e.eat(">"))return y("=>","operator")
if("0"==i&&e.eat(/x/i))return e.eatWhile(/[\da-f]/i),y("number","number")
if("0"==i&&e.eat(/o/i))return e.eatWhile(/[0-7]/i),y("number","number")
if("0"==i&&e.eat(/b/i))return e.eatWhile(/[01]/i),y("number","number")
if(/\d/.test(i))return e.match(/^\d*(?:\.\d*)?(?:[eE][+\-]?\d+)?/),y("number","number")
if("/"==i)return e.eat("*")?(r.tokenize=v,v(e,r)):e.eat("/")?(e.skipToEnd(),y("comment","comment")):t(e,r,1)?(function(e){for(var t,r=!1,n=!1;null!=(t=e.next());){if(!r){if("/"==t&&!n)return
"["==t?n=!0:n&&"]"==t&&(n=!1)}r=!r&&"\\"==t}}(e),e.match(/^\b(([gimyu])(?![gimyu]*\2))+\b/),y("regexp","string-2")):(e.eatWhile(p),y("operator","operator",e.current()))
if("`"==i)return r.tokenize=g,g(e,r)
if("#"==i)return e.skipToEnd(),y("error","error")
if(p.test(i))return e.eatWhile(p),y("operator","operator",e.current())
if(f.test(i)){e.eatWhile(f)
var a=e.current(),s=h.propertyIsEnumerable(a)&&h[a]
return s&&"."!=r.lastType?y(s.type,s.style,a):y("variable","variable",a)}}function v(e,t){for(var r,n=!1;r=e.next();){if("/"==r&&n){t.tokenize=m
break}n="*"==r}return y("comment","comment")}function g(e,t){for(var r,n=!1;null!=(r=e.next());){if(!n&&("`"==r||"$"==r&&e.eat("{"))){t.tokenize=m
break}n=!n&&"\\"==r}return y("quasi","string-2",e.current())}function b(e,t){t.fatArrowAt&&(t.fatArrowAt=null)
var r=e.string.indexOf("=>",e.start)
if(!(r<0)){for(var n=0,i=!1,a=r-1;a>=0;--a){var s=e.string.charAt(a),o="([{}])".indexOf(s)
if(o>=0&&o<3){if(!n){++a
break}if(0==--n)break}else if(o>=3&&o<6)++n
else if(f.test(s))i=!0
else{if(/["'\/]/.test(s))return
if(i&&!n){++a
break}}}i&&!n&&(t.fatArrowAt=a)}}var x={atom:!0,number:!0,variable:!0,string:!0,regexp:!0,this:!0,"jsonld-keyword":!0}
function k(e,t,r,n,i,a){this.indented=e,this.column=t,this.type=r,this.prev=i,this.info=a,null!=n&&(this.align=n)}function w(e,t){for(var r=e.localVars;r;r=r.next)if(r.name==t)return!0
for(var n=e.context;n;n=n.prev)for(r=n.vars;r;r=r.next)if(r.name==t)return!0}var _={state:null,column:null,marked:null,cc:null}
function E(){for(var e=arguments.length-1;e>=0;e--)_.cc.push(arguments[e])}function j(){return E.apply(null,arguments),!0}function S(e){function t(t){for(var r=t;r;r=r.next)if(r.name==e)return!0
return!1}var r=_.state
if(_.marked="def",r.context){if(t(r.localVars))return
r.localVars={name:e,next:r.localVars}}else{if(t(r.globalVars))return
n.globalVars&&(r.globalVars={name:e,next:r.globalVars})}}var I={name:"this",next:{name:"arguments"}}
function $(){_.state.context={prev:_.state.context,vars:_.state.localVars},_.state.localVars=I}function A(){_.state.localVars=_.state.context.vars,_.state.context=_.state.context.prev}function M(e,t){var r=function(){var r=_.state,n=r.indented
if("stat"==r.lexical.type)n=r.lexical.indented
else for(var i=r.lexical;i&&")"==i.type&&i.align;i=i.prev)n=i.indented
r.lexical=new k(n,_.stream.column(),e,null,r.lexical,t)}
return r.lex=!0,r}function N(){var e=_.state
e.lexical.prev&&(")"==e.lexical.type&&(e.indented=e.lexical.indented),e.lexical=e.lexical.prev)}function O(e){return function t(r){return r==e?j():";"==e?E():j(t)}}function V(e,t){return"var"==e?j(M("vardef",t.length),ae,O(";"),N):"keyword a"==e?j(M("form"),T,V,N):"keyword b"==e?j(M("form"),V,N):"{"==e?j(M("}"),ee,N):";"==e?j():"if"==e?("else"==_.state.lexical.info&&_.state.cc[_.state.cc.length-1]==N&&_.state.cc.pop()(),j(M("form"),T,V,N,ue)):"function"==e?j(me):"for"==e?j(M("form"),fe,V,N):"variable"==e?j(M("stat"),D):"switch"==e?j(M("form"),T,M("}","switch"),O("{"),ee,N,N):"case"==e?j(T,O(":")):"default"==e?j(O(":")):"catch"==e?j(M("form"),$,O("("),ve,O(")"),V,N,A):"class"==e?j(M("form"),ge,N):"export"==e?j(M("stat"),we,N):"import"==e?j(M("stat"),_e,N):"module"==e?j(M("form"),se,M("}"),O("{"),ee,N,N):"async"==e?j(V):E(M("stat"),T,O(";"),N)}function T(e){return L(e,!1)}function z(e){return L(e,!0)}function L(e,t){if(_.state.fatArrowAt==_.stream.start){var r=t?W:C
if("("==e)return j($,M(")"),Y(se,")"),N,O("=>"),r,A)
if("variable"==e)return E($,se,O("=>"),r,A)}var n=t?F:J
return x.hasOwnProperty(e)?j(n):"function"==e?j(me,n):"keyword c"==e?j(t?P:q):"("==e?j(M(")"),q,Ae,O(")"),N,n):"operator"==e||"spread"==e?j(t?z:T):"["==e?j(M("]"),Ie,N,n):"{"==e?Z(K,"}",null,n):"quasi"==e?E(U,n):"new"==e?j(function(e){return function(t){return"."==t?j(e?G:B):E(e?z:T)}}(t)):j()}function q(e){return e.match(/[;\}\)\],]/)?E():E(T)}function P(e){return e.match(/[;\}\)\],]/)?E():E(z)}function J(e,t){return","==e?j(T):F(e,t,!1)}function F(e,t,r){var n=0==r?J:F,i=0==r?T:z
return"=>"==e?j($,r?W:C,A):"operator"==e?/\+\+|--/.test(t)?j(n):"?"==t?j(T,O(":"),i):j(i):"quasi"==e?E(U,n):";"!=e?"("==e?Z(z,")","call",n):"."==e?j(H,n):"["==e?j(M("]"),q,O("]"),N,n):void 0:void 0}function U(e,t){return"quasi"!=e?E():"${"!=t.slice(t.length-2)?j(U):j(T,R)}function R(e){if("}"==e)return _.marked="string-2",_.state.tokenize=g,j(U)}function C(e){return b(_.stream,_.state),E("{"==e?V:T)}function W(e){return b(_.stream,_.state),E("{"==e?V:z)}function B(e,t){if("target"==t)return _.marked="keyword",j(J)}function G(e,t){if("target"==t)return _.marked="keyword",j(F)}function D(e){return":"==e?j(N,V):E(J,O(";"),N)}function H(e){if("variable"==e)return _.marked="property",j()}function K(e,t){return"variable"==e||"keyword"==_.style?(_.marked="property",j("get"==t||"set"==t?Q:X)):"number"==e||"string"==e?(_.marked=l?"property":_.style+" property",j(X)):"jsonld-keyword"==e?j(X):"modifier"==e?j(K):"["==e?j(T,O("]"),X):"spread"==e?j(T):void 0}function Q(e){return"variable"!=e?E(X):(_.marked="property",j(me))}function X(e){return":"==e?j(z):"("==e?E(me):void 0}function Y(e,t){function r(n,i){if(","==n){var a=_.state.lexical
return"call"==a.info&&(a.pos=(a.pos||0)+1),j(e,r)}return n==t||i==t?j():j(O(t))}return function(n,i){return n==t||i==t?j():E(e,r)}}function Z(e,t,r){for(var n=3;n<arguments.length;n++)_.cc.push(arguments[n])
return j(M(t,r),Y(e,t),N)}function ee(e){return"}"==e?j():E(V,ee)}function te(e){if(u&&":"==e)return j(ne)}function re(e,t){if("="==t)return j(z)}function ne(e){if("variable"==e)return _.marked="variable-3",j(ie)}function ie(e,t){return"<"==t?j(Y(ne,">"),ie):"["==e?j(O("]"),ie):void 0}function ae(){return E(se,te,le,ce)}function se(e,t){return"modifier"==e?j(se):"variable"==e?(S(t),j()):"spread"==e?j(se):"["==e?Z(se,"]"):"{"==e?Z(oe,"}"):void 0}function oe(e,t){return"variable"!=e||_.stream.match(/^\s*:/,!1)?("variable"==e&&(_.marked="property"),"spread"==e?j(se):"}"==e?E():j(O(":"),se,le)):(S(t),j(le))}function le(e,t){if("="==t)return j(z)}function ce(e){if(","==e)return j(ae)}function ue(e,t){if("keyword b"==e&&"else"==t)return j(M("form","else"),V,N)}function fe(e){if("("==e)return j(M(")"),he,O(")"),N)}function he(e){return"var"==e?j(ae,O(";"),de):";"==e?j(de):"variable"==e?j(pe):E(T,O(";"),de)}function pe(e,t){return"in"==t||"of"==t?(_.marked="keyword",j(T)):j(J,de)}function de(e,t){return";"==e?j(ye):"in"==t||"of"==t?(_.marked="keyword",j(T)):E(T,O(";"),ye)}function ye(e){")"!=e&&j(T)}function me(e,t){return"*"==t?(_.marked="keyword",j(me)):"variable"==e?(S(t),j(me)):"("==e?j($,M(")"),Y(ve,")"),N,te,V,A):void 0}function ve(e){return"spread"==e?j(ve):E(se,te,re)}function ge(e,t){if("variable"==e)return S(t),j(be)}function be(e,t){return"extends"==t?j(T,be):"{"==e?j(M("}"),xe,N):void 0}function xe(e,t){return"variable"==e||"keyword"==_.style?"static"==t?(_.marked="keyword",j(xe)):(_.marked="property","get"==t||"set"==t?j(ke,me,xe):j(me,xe)):"*"==t?(_.marked="keyword",j(xe)):";"==e?j(xe):"}"==e?j():void 0}function ke(e){return"variable"!=e?E():(_.marked="property",j())}function we(e,t){return"*"==t?(_.marked="keyword",j(Se,O(";"))):"default"==t?(_.marked="keyword",j(T,O(";"))):E(V)}function _e(e){return"string"==e?j():E(Ee,Se)}function Ee(e,t){return"{"==e?Z(Ee,"}"):("variable"==e&&S(t),"*"==t&&(_.marked="keyword"),j(je))}function je(e,t){if("as"==t)return _.marked="keyword",j(Ee)}function Se(e,t){if("from"==t)return _.marked="keyword",j(T)}function Ie(e){return"]"==e?j():E(z,$e)}function $e(e){return"for"==e?E(Ae,O("]")):","==e?j(Y(P,"]")):E(Y(z,"]"))}function Ae(e){return"for"==e?j(fe,Ae):"if"==e?j(T,Ae):void 0}return N.lex=!0,{startState:function(e){var t={tokenize:m,lastType:"sof",cc:[],lexical:new k((e||0)-s,0,"block",!1),localVars:n.localVars,context:n.localVars&&{vars:n.localVars},indented:e||0}
return n.globalVars&&"object"==typeof n.globalVars&&(t.globalVars=n.globalVars),t},token:function(e,t){if(e.sol()&&(t.lexical.hasOwnProperty("align")||(t.lexical.align=!1),t.indented=e.indentation(),b(e,t)),t.tokenize!=v&&e.eatSpace())return null
var r=t.tokenize(e,t)
return"comment"==i?r:(t.lastType="operator"!=i||"++"!=a&&"--"!=a?i:"incdec",function(e,t,r,n,i){var a=e.cc
for(_.state=e,_.stream=i,_.marked=null,_.cc=a,_.style=t,e.lexical.hasOwnProperty("align")||(e.lexical.align=!0);;){if((a.length?a.pop():c?T:V)(r,n)){for(;a.length&&a[a.length-1].lex;)a.pop()()
return _.marked?_.marked:"variable"==r&&w(e,n)?"variable-2":t}}}(t,r,i,a,e))},indent:function(t,r){if(t.tokenize==v)return e.Pass
if(t.tokenize!=m)return 0
var i=r&&r.charAt(0),a=t.lexical
if(!/^\s*else\b/.test(r))for(var l=t.cc.length-1;l>=0;--l){var c=t.cc[l]
if(c==N)a=a.prev
else if(c!=ue)break}"stat"==a.type&&"}"==i&&(a=a.prev),o&&")"==a.type&&"stat"==a.prev.type&&(a=a.prev)
var u=a.type,f=i==u
return"vardef"==u?a.indented+("operator"==t.lastType||","==t.lastType?a.info+1:0):"form"==u&&"{"==i?a.indented:"form"==u?a.indented+s:"stat"==u?a.indented+(function(e,t){return"operator"==e.lastType||","==e.lastType||p.test(t.charAt(0))||/[,.]/.test(t.charAt(0))}(t,r)?o||s:0):"switch"!=a.info||f||0==n.doubleIndentSwitch?a.align?a.column+(f?0:1):a.indented+(f?0:s):a.indented+(/^(?:case|default)\b/.test(r)?s:2*s)},electricInput:/^\s*(?:case .*?:|default:|\{|\})$/,blockCommentStart:c?null:"/*",blockCommentEnd:c?null:"*/",lineComment:c?null:"//",fold:"brace",closeBrackets:"()[]{}''\"\"``",helperType:c?"json":"javascript",jsonldMode:l,jsonMode:c,expressionAllowed:t,skipExpression:function(e){var t=e.cc[e.cc.length-1]
t!=T&&t!=z||e.cc.pop()}}})),e.registerHelper("wordChars","javascript",/[\w$]/),e.defineMIME("text/javascript","javascript"),e.defineMIME("text/ecmascript","javascript"),e.defineMIME("application/javascript","javascript"),e.defineMIME("application/x-javascript","javascript"),e.defineMIME("application/ecmascript","javascript"),e.defineMIME("application/json",{name:"javascript",json:!0}),e.defineMIME("application/x-json",{name:"javascript",json:!0}),e.defineMIME("application/ld+json",{name:"javascript",jsonld:!0}),e.defineMIME("text/typescript",{name:"javascript",typescript:!0}),e.defineMIME("application/typescript",{name:"javascript",typescript:!0})}))
