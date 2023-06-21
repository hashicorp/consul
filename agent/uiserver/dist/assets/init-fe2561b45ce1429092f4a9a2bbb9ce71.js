(function(e,t){const n=new Map(Object.entries(JSON.parse(e.querySelector("[data-consul-ui-fs]").textContent))),o=function(t){var n=e.createElement("script")
n.src=t,e.body.appendChild(n)}
"TextDecoder"in window||(o(n.get(`${["text-encoding","encoding-indexes"].join("/")}.js`)),o(n.get(`${["text-encoding","encoding"].join("/")}.js`))),window.CSS&&window.CSS.escape||o(n.get(`${["css.escape","css.escape"].join("/")}.js`))
try{const t=e.querySelector('[name="consul-ui/config/environment"]'),n=JSON.parse(e.querySelector("[data-consul-ui-config]").textContent),o=JSON.parse(decodeURIComponent(t.getAttribute("content"))),c="string"!=typeof n.ContentPath?"":n.ContentPath
c.length>0&&(o.rootURL=c),t.setAttribute("content",encodeURIComponent(JSON.stringify(o)))}catch(c){throw new Error(`Unable to parse consul-ui settings: ${c.message}`)}})(document)
