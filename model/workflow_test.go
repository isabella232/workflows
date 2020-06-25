// Copyright 2020 Northern.tech AS
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package model

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseWorkflowFromJSON(t *testing.T) {
	data := []byte(`
{
	"name": "decommission_device",
	"description": "Removes device info from all services.",
	"version": 4,
	"tasks": [
		{
			"name": "delete_device_inventory",
			"type": "http",
			"http": {
				"uri": "http://mender-inventory:8080/api/0.1.0/devices/${workflow.input.device_id}",
				"method": "DELETE",
				"body": "Payload",
				"headers": {
					"X-MEN-RequestID": "${workflow.input.request_id}",
					"Authorization": "${workflow.input.authorization}"
				},
				"statusCodes": [
					200,
					201
				],
				"connectionTimeOut": 1000,
				"readTimeOut": 1000
			}
		}
	],
	"inputParameters": [
		"device_id",
		"request_id",
		"authorization"
	],
	"schemaVersion": 1
}`)

	var workflow, err = ParseWorkflowFromJSON(data)
	assert.Nil(t, err)
	assert.NotNil(t, workflow)
	assert.Equal(t, "decommission_device", workflow.Name)
	assert.Equal(t, "Removes device info from all services.", workflow.Description)
	assert.Equal(t, 4, workflow.Version)
	assert.Equal(t, 1, workflow.SchemaVersion)

	var inputParameters = workflow.InputParameters
	assert.Len(t, inputParameters, 3)
	assert.Equal(t, inputParameters[0], "device_id")
	assert.Equal(t, inputParameters[1], "request_id")
	assert.Equal(t, inputParameters[2], "authorization")

	var tasks = workflow.Tasks
	assert.Len(t, tasks, 1)
	assert.Equal(t, tasks[0].Name, "delete_device_inventory")
	assert.Equal(t, tasks[0].Type, TaskTypeHTTP)

	var httpTask *HTTPTask = tasks[0].HTTP
	assert.Equal(t, httpTask.URI, "http://mender-inventory:8080/api/0.1.0/devices/${workflow.input.device_id}")
	assert.Equal(t, httpTask.Method, "DELETE")
	assert.Equal(t, httpTask.Body, "Payload")
	assert.Len(t, httpTask.Headers, 2)
	assert.Len(t, httpTask.StatusCodes, 2)
	assert.Equal(t, httpTask.StatusCodes[0], 200)
	assert.Equal(t, httpTask.StatusCodes[1], 201)
	assert.Equal(t, httpTask.Headers["X-MEN-RequestID"], "${workflow.input.request_id}")
	assert.Equal(t, httpTask.Headers["Authorization"], "${workflow.input.authorization}")
	assert.Equal(t, httpTask.ConnectionTimeOut, 1000)
	assert.Equal(t, httpTask.ReadTimeOut, 1000)
}

func TestParseWorkflowWithCLIFromJSON(t *testing.T) {
	data := []byte(`
{
	"name": "test_cli",
	"description": "Test CLI",
	"version": 1,
	"tasks": [
		{
			"name": "run_echo",
			"type": "cli",
			"cli": {
				"command": [
					"echo",
					"1"
				],
				"executionTimeOut": 1000
			}
		}
	],
	"schemaVersion": 1
}`)

	var workflow, err = ParseWorkflowFromJSON(data)
	assert.Nil(t, err)
	assert.NotNil(t, workflow)
	assert.Equal(t, "test_cli", workflow.Name)
	assert.Equal(t, "Test CLI", workflow.Description)
	assert.Equal(t, 1, workflow.Version)
	assert.Equal(t, 1, workflow.SchemaVersion)

	var tasks = workflow.Tasks
	assert.Len(t, tasks, 1)
	assert.Equal(t, tasks[0].Name, "run_echo")
	assert.Equal(t, tasks[0].Type, TaskTypeCLI)

	var cliTask *CLITask = tasks[0].CLI
	assert.Equal(t, cliTask.Command, []string{"echo", "1"})
	assert.Equal(t, cliTask.ExecutionTimeOut, 1000)
}

func TestParseWorkflowFromInvalidJSON(t *testing.T) {
	data := []byte(`INVALID JSON`)

	var workflow, err = ParseWorkflowFromJSON(data)
	assert.Nil(t, workflow)
	assert.NotNil(t, err)
}

func TestGetWorkflowsFromPath(t *testing.T) {
	testCases := []struct {
		Name string

		//WorkflowFiles maps filenames to file contents
		WorkflowFiles map[string]string
		NumSuccessful int
	}{{
		Name:          "Successful JSON and YAML",
		NumSuccessful: 2,
		WorkflowFiles: map[string]string{
			"decommission.json": `{
	"name": "decommission_device",
	"description": "Removes device info from all services.",
	"version": 4,
	"tasks": [
		{
			"name": "delete_device_inventory",
			"type": "http",
			"http": {
				"uri": "http://mender-inventory:8080/api/0.1.0/devices/${workflow.input.device_id}",
				"method": "DELETE",
				"body": "Payload",
				"headers": {
					"X-MEN-RequestID": "${workflow.input.request_id}",
					"Authorization": "${workflow.input.authorization}"
				},
				"connectionTimeOut": 1000,
				"readTimeOut": 1000
			}
		}
	],
	"inputParameters": [
		"device_id",
		"request_id",
		"authorization"
	],
	"schemaVersion": 1
}`,
			"provision.yaml": `
name: provision_device
description: Provision device.
version: 2
tasks:
  - name: create_device_inventory
    type: http
    retries: 3
    http:
      uri: http://mender-inventory:8080/api/internal/v1/inventory/devices
      method: POST
      contentType: application/json
      body: ${workflow.input.device}
      headers:
        X-MEN-RequestID: ${workflow.input.request_id}
        Authorization: ${workflow.input.authorization}
      connectionTimeOut: 8000
      readTimeOut: 8000
inputParameters:
  - request_id
  - authorization
  - device
`,
		},
	}, {
		Name: "Bad JSON",

		WorkflowFiles: map[string]string{
			"fail.json": "{{}",
		},
	}, {
		Name: "Bad YAML",

		WorkflowFiles: map[string]string{
			"fail.yml": `  foo: bar: baz`,
		},
	}, {
		Name: "One good / one bad / one non-workflow file",

		NumSuccessful: 1,
		WorkflowFiles: map[string]string{
			"good.yaml": `
name: provision_device
description: Provision device.
version: 2
tasks:
  - name: create_device_inventory
    type: http
    retries: 3
    http:
      uri: http://mender-inventory:8080/api/internal/v1/inventory/devices
      method: POST
      contentType: application/json
      body: ${workflow.input.device}
      headers:
        X-MEN-RequestID: ${workflow.input.request_id}
        Authorization: ${workflow.input.authorization}
      connectionTimeOut: 8000
      readTimeOut: 8000
inputParameters:
  - request_id
  - authorization
  - device
`,
			"bad.json":    `{foo: "bar"}`,
			"random.file": "random content",
		},
	}}

	t.Parallel()
	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			dir, _ := ioutil.TempDir("", "go_test_workflows")
			defer os.RemoveAll(dir)
			for filename, contents := range testCase.WorkflowFiles {
				tmpfn := filepath.Join(dir, filename)
				ioutil.WriteFile(tmpfn, []byte(contents), 0666)

			}
			workflows := GetWorkflowsFromPath(dir)
			assert.Len(t,
				workflows,
				testCase.NumSuccessful,
			)
			for name, workflow := range workflows {
				assert.Equal(
					t, name, workflow.Name,
				)
			}
		})
	}
}
func TestGetWorkflowsFromPathNoSuchDirectory(t *testing.T) {
	workflows := GetWorkflowsFromPath("/tmp/path/to/directory/that/does/not/exist/at/all")
	assert.Len(t, workflows, 0)
}
