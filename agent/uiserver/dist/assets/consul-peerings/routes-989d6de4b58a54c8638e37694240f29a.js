/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

((e,s=("undefined"!=typeof document?document.currentScript.dataset:module.exports))=>{s.routes=JSON.stringify(e)})({dc:{peers:{_options:{path:"/peers"},index:{_options:{path:"/",queryParams:{sortBy:"sort",state:"state",searchproperty:{as:"searchproperty",empty:[["Name","ID"]]},search:{as:"filter",replace:!0}}}},show:{_options:{path:"/:name"},imported:{_options:{path:"/imported-services",queryParams:{sortBy:"sort",status:"status",source:"source",kind:"kind",searchproperty:{as:"searchproperty",empty:[["Name","Tags"]]},search:{as:"filter",replace:!0}}}},exported:{_options:{path:"/exported-services",queryParams:{search:{as:"filter",replace:!0}}}},addresses:{_options:{path:"/addresses"}}}}}})
