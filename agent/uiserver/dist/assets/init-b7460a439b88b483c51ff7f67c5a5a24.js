(function(e,t){const n=new Map(Object.entries(JSON.parse(e.querySelector(`[data-${t}-fs]`).textContent))),o=function(t){var n=e.createElement("script")
n.src=t,e.body.appendChild(n)}
"TextDecoder"in window||(o(n.get(`${["text-encoding","encoding-indexes"].join("/")}.js`)),o(n.get(`${["text-encoding","encoding"].join("/")}.js`))),window.CSS&&window.CSS.escape||o(n.get(`${["css.escape","css.escape"].join("/")}.js`))
try{const n=e.querySelector(`[name="${t}/config/environment"]`),o=JSON.parse(e.querySelector(`[data-${t}-config]`).textContent),c=JSON.parse(decodeURIComponent(n.getAttribute("content"))),s="string"!=typeof o.ContentPath?"":o.ContentPath
s.length>0&&(c.rootURL=s),n.setAttribute("content",encodeURIComponent(JSON.stringify(c)))}catch(c){throw new Error(`Unable to parse ${t} settings: ${c.message}`)}})(document,"consul-ui")
