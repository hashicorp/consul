// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// gnostic_go_generator is a sample Gnostic plugin that generates Go
// code that supports an API.
package main

import (
	"encoding/json"
	"errors"
	"strings"

	plugins "github.com/googleapis/gnostic/plugins"
)

// This is the main function for the code generation plugin.
func main() {
	env, err := plugins.NewEnvironment()
	env.RespondAndExitIfError(err)

	packageName := env.Request.OutputPath

	// Use the name used to run the plugin to decide which files to generate.
	var files []string
	switch {
	case strings.Contains(env.Invocation, "gnostic-go-client"):
		files = []string{"client.go", "types.go", "constants.go"}
	case strings.Contains(env.Invocation, "gnostic-go-server"):
		files = []string{"server.go", "provider.go", "types.go", "constants.go"}
	default:
		files = []string{"client.go", "server.go", "provider.go", "types.go", "constants.go"}
	}

	// Get the code surface model.
	model := env.Request.Surface

	if model == nil {
		err = errors.New("No generated code surface model is available.")
		env.RespondAndExitIfError(err)
	}

	// Customize the code surface model for Go
	NewGoLanguageModel().Prepare(model)

	modelJSON, _ := json.MarshalIndent(model, "", "  ")
	modelFile := &plugins.File{Name: "model.json", Data: modelJSON}
	env.Response.Files = append(env.Response.Files, modelFile)

	// Create the renderer.
	renderer, err := NewServiceRenderer(model)
	renderer.Package = packageName
	env.RespondAndExitIfError(err)

	// Run the renderer to generate files and add them to the response object.
	err = renderer.Render(env.Response, files)
	env.RespondAndExitIfError(err)

	// Return with success.
	env.RespondAndExit()
}
