package agent

func compileWatchParametersForArchetype(config Config, key string) map[string]interface{} {
	params := make(map[string]interface{})
	params["type"] = "key"
	params["datacenter"] = config.Datacenter
	params["token"] = config.ACLToken
	params["key"] = key
	return params
}
