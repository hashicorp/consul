/*
Package stacks provides operation for working with Heat stacks. A stack is a
group of resources (servers, load balancers, databases, and so forth)
combined to fulfill a useful purpose. Based on a template, Heat orchestration
engine creates an instantiated set of resources (a stack) to run the
application framework or component specified (in the template). A stack is a
running instance of a template. The result of creating a stack is a deployment
of the application framework or component.

Summary of  Behavior Between Stack Update and UpdatePatch Methods :

Function | Test Case | Result

Update()	| Template AND Parameters WITH Conflict | Parameter takes priority, parameters are set in raw_template.environment overlay
Update()	| Template ONLY | Template updates, raw_template.environment overlay is removed
Update()	| Parameters ONLY | No update, template is required

UpdatePatch() 	| Template AND Parameters WITH Conflict | Parameter takes priority, parameters are set in raw_template.environment overlay
UpdatePatch() 	| Template ONLY | Template updates, but raw_template.environment overlay is not removed, existing parameter values will remain
UpdatePatch() 	| Parameters ONLY | Parameters (raw_template.environment) is updated, excluded values are unchanged

The PUT Update() function will remove parameters from the raw_template.environment overlay
if they are excluded from the operation, whereas PATCH Update() will never be destructive to the
raw_template.environment overlay.  It is not possible to expose the raw_template values with a
patch update once they have been added to the environment overlay with the PATCH verb, but
newly added values that do not have a corresponding key in the overlay will display the
raw_template value.

Example to Update a Stack Using the Update (PUT) Method

	t := make(map[string]interface{})
	f, err := ioutil.ReadFile("template.yaml")
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(f, t)
	if err != nil {
		panic(err)
	}

	template := stacks.Template{}
	template.TE = stacks.TE{
		Bin: f,
	}

	var params = make(map[string]interface{})
	params["number_of_nodes"] = 2

	stackName := "my_stack"
	stackId := "d68cc349-ccc5-4b44-a17d-07f068c01e5a"

	stackOpts := &stacks.UpdateOpts{
		Parameters: params,
		TemplateOpts: &template,
	}

	res := stacks.Update(orchestrationClient, stackName, stackId, stackOpts)
	if res.Err != nil {
		panic(res.Err)
	}

Example to Update a Stack Using the UpdatePatch (PATCH) Method

	var params = make(map[string]interface{})
	params["number_of_nodes"] = 2

	stackName := "my_stack"
	stackId := "d68cc349-ccc5-4b44-a17d-07f068c01e5a"

	stackOpts := &stacks.UpdateOpts{
		Parameters: params,
	}

	res := stacks.UpdatePatch(orchestrationClient, stackName, stackId, stackOpts)
	if res.Err != nil {
		panic(res.Err)
	}

Example YAML Template Containing a Heat::ResourceGroup With Three Nodes

	heat_template_version: 2016-04-08

	parameters:
		number_of_nodes:
			type: number
			default: 3
			description: the number of nodes
		node_flavor:
			type: string
			default: m1.small
			description: node flavor
		node_image:
			type: string
			default: centos7.5-latest
			description: node os image
		node_network:
			type: string
			default: my-node-network
			description: node network name

	resources:
		resource_group:
			type: OS::Heat::ResourceGroup
			properties:
			count: { get_param: number_of_nodes }
			resource_def:
				type: OS::Nova::Server
				properties:
					name: my_nova_server_%index%
					image: { get_param: node_image }
					flavor: { get_param: node_flavor }
					networks:
						- network: {get_param: node_network}
*/
package stacks
