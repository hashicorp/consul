(function(r){const e=["init","serviceRecentSummarySeries","serviceRecentSummaryStats","upstreamRecentSummaryStats","downstreamRecentSummaryStats"]
r.consul=new class{constructor(){this.registry={},this.providers={}}registerMetricsProvider(r,t){for(var i of e)if("function"!=typeof t[i])throw new Error(`Can't register metrics provider '${r}': missing ${i} method.`)
this.registry[r]=t}getMetricsProvider(r,e){if(!(r in this.registry))throw new Error(`Metrics Provider '${r}' is not registered.`)
return r in this.providers||(this.providers[r]=Object.create(this.registry[r]),this.providers[r].init(e)),this.providers[r]}}})(window)
