package xds

import (
	"github.com/hashicorp/consul/agent/xds/builtinextensions/lambda"
	"github.com/hashicorp/consul/agent/xds/builtinextensions/lua"
	"github.com/hashicorp/consul/agent/xds/builtinextensiontemplate"
	"github.com/hashicorp/consul/agent/xds/xdscommon"
	"github.com/hashicorp/consul/api"
)

func GetBuiltInExtension(ext xdscommon.ExtensionConfiguration) (builtinextensiontemplate.EnvoyExtension, bool) {
	var c builtinextensiontemplate.PluginConstructor
	switch ext.EnvoyExtension.Name {
	case api.BuiltinAWSLambdaExtension:
		c = lambda.MakeLambdaExtension
	case api.BuiltinLuaExtension:
		c = lua.MakeLuaExtension
	default:
		var e builtinextensiontemplate.EnvoyExtension
		return e, false
	}

	return builtinextensiontemplate.EnvoyExtension{Constructor: c}, true
}
