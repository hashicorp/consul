/**
 * Copyright (c) HashiCorp, Inc.
 * SPDX-License-Identifier: MPL-2.0
 */

((e,t=("undefined"!=typeof document?document.currentScript.dataset:module.exports))=>{t.routes=JSON.stringify(e)})({dc:{nspaces:{_options:{path:"/namespaces",abilities:["read nspaces"]},index:{_options:{path:"/",queryParams:{sortBy:"sort",searchproperty:{as:"searchproperty",empty:[["Name","Description","Role","Policy"]]},search:{as:"filter",replace:!0}}}},edit:{_options:{path:"/:name"}},create:{_options:{template:"../edit",path:"/create",abilities:["create nspaces"]}}}}})
