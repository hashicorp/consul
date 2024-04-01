/*! https://mths.be/cssescape v1.5.1 by @mathias | MIT license */
(function(e,t){"object"==typeof exports?module.exports=t(e):"function"==typeof define&&define.amd?define([],t.bind(e,e)):t(e)})("undefined"!=typeof global?global:this,(function(e){if(e.CSS&&e.CSS.escape)return e.CSS.escape
var t=function(e){if(0==arguments.length)throw new TypeError("`CSS.escape` requires an argument.")
for(var t,n=String(e),r=n.length,o=-1,S="",a=n.charCodeAt(0);++o<r;)0!=(t=n.charCodeAt(o))?S+=t>=1&&t<=31||127==t||0==o&&t>=48&&t<=57||1==o&&t>=48&&t<=57&&45==a?"\\"+t.toString(16)+" ":(0!=o||1!=r||45!=t)&&(t>=128||45==t||95==t||t>=48&&t<=57||t>=65&&t<=90||t>=97&&t<=122)?n.charAt(o):"\\"+n.charAt(o):S+="�"
return S}
return e.CSS||(e.CSS={}),e.CSS.escape=t,t}))
