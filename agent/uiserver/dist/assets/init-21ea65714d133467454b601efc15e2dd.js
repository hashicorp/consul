(function(e){const t=new Map(Object.entries(JSON.parse(e.querySelector("[data-consul-ui-fs]").textContent))),n=function(t){var n=e.createElement("script")
n.src=t,e.body.appendChild(n)}
"TextDecoder"in window||(n(t.get(["text-encoding","encoding-indexes"].join("/")+".js")),n(t.get(["text-encoding","encoding"].join("/")+".js"))),window.CSS&&window.CSS.escape||n(t.get(["css.escape","css.escape"].join("/")+".js"))
try{const t=e.querySelector('[name="consul-ui/config/environment"]'),n=JSON.parse(e.querySelector("[data-consul-ui-config]").textContent),o=JSON.parse(decodeURIComponent(t.getAttribute("content"))),c="string"!=typeof n.ContentPath?"":n.ContentPath
c.length>0&&(o.rootURL=c),t.setAttribute("content",encodeURIComponent(JSON.stringify(o)))}catch(o){throw new Error("Unable to parse consul-ui settings: "+o.message)}})(document)
