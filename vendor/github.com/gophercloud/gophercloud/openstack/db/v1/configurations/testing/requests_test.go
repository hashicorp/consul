package testing

import (
	"testing"

	"github.com/gophercloud/gophercloud/openstack/db/v1/configurations"
	"github.com/gophercloud/gophercloud/openstack/db/v1/instances"
	"github.com/gophercloud/gophercloud/pagination"
	th "github.com/gophercloud/gophercloud/testhelper"
	fake "github.com/gophercloud/gophercloud/testhelper/client"
	"github.com/gophercloud/gophercloud/testhelper/fixture"
)

var (
	configID = "{configID}"
	_baseURL = "/configurations"
	resURL   = _baseURL + "/" + configID

	dsID               = "{datastoreID}"
	versionID          = "{versionID}"
	paramID            = "{paramID}"
	dsParamListURL     = "/datastores/" + dsID + "/versions/" + versionID + "/parameters"
	dsParamGetURL      = "/datastores/" + dsID + "/versions/" + versionID + "/parameters/" + paramID
	globalParamListURL = "/datastores/versions/" + versionID + "/parameters"
	globalParamGetURL  = "/datastores/versions/" + versionID + "/parameters/" + paramID
)

func TestList(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()
	fixture.SetupHandler(t, _baseURL, "GET", "", ListConfigsJSON, 200)

	count := 0
	err := configurations.List(fake.ServiceClient()).EachPage(func(page pagination.Page) (bool, error) {
		count++
		actual, err := configurations.ExtractConfigs(page)
		th.AssertNoErr(t, err)

		expected := []configurations.Config{ExampleConfig}
		th.AssertDeepEquals(t, expected, actual)

		return true, nil
	})

	th.AssertEquals(t, 1, count)
	th.AssertNoErr(t, err)
}

func TestGet(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()
	fixture.SetupHandler(t, resURL, "GET", "", GetConfigJSON, 200)

	config, err := configurations.Get(fake.ServiceClient(), configID).Extract()
	th.AssertNoErr(t, err)
	th.AssertDeepEquals(t, &ExampleConfig, config)
}

func TestCreate(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()
	fixture.SetupHandler(t, _baseURL, "POST", CreateReq, CreateConfigJSON, 200)

	opts := configurations.CreateOpts{
		Datastore: &configurations.DatastoreOpts{
			Type:    "a00000a0-00a0-0a00-00a0-000a000000aa",
			Version: "b00000b0-00b0-0b00-00b0-000b000000bb",
		},
		Description: "example description",
		Name:        "example-configuration-name",
		Values: map[string]interface{}{
			"collation_server": "latin1_swedish_ci",
			"connect_timeout":  120,
		},
	}

	config, err := configurations.Create(fake.ServiceClient(), opts).Extract()
	th.AssertNoErr(t, err)
	th.AssertDeepEquals(t, &ExampleConfigWithValues, config)
}

func TestUpdate(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()
	fixture.SetupHandler(t, resURL, "PATCH", UpdateReq, "", 200)

	opts := configurations.UpdateOpts{
		Values: map[string]interface{}{
			"connect_timeout": 300,
		},
	}

	err := configurations.Update(fake.ServiceClient(), configID, opts).ExtractErr()
	th.AssertNoErr(t, err)
}

func TestReplace(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()
	fixture.SetupHandler(t, resURL, "PUT", UpdateReq, "", 202)

	opts := configurations.UpdateOpts{
		Values: map[string]interface{}{
			"connect_timeout": 300,
		},
	}

	err := configurations.Replace(fake.ServiceClient(), configID, opts).ExtractErr()
	th.AssertNoErr(t, err)
}

func TestDelete(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()
	fixture.SetupHandler(t, resURL, "DELETE", "", "", 202)

	err := configurations.Delete(fake.ServiceClient(), configID).ExtractErr()
	th.AssertNoErr(t, err)
}

func TestListInstances(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()
	fixture.SetupHandler(t, resURL+"/instances", "GET", "", ListInstancesJSON, 200)

	expectedInstance := instances.Instance{
		ID:   "d4603f69-ec7e-4e9b-803f-600b9205576f",
		Name: "json_rack_instance",
	}

	pages := 0
	err := configurations.ListInstances(fake.ServiceClient(), configID).EachPage(func(page pagination.Page) (bool, error) {
		pages++

		actual, err := instances.ExtractInstances(page)
		if err != nil {
			return false, err
		}

		th.AssertDeepEquals(t, actual, []instances.Instance{expectedInstance})

		return true, nil
	})

	th.AssertNoErr(t, err)
	th.AssertEquals(t, 1, pages)
}

func TestListDSParams(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()
	fixture.SetupHandler(t, dsParamListURL, "GET", "", ListParamsJSON, 200)

	pages := 0
	err := configurations.ListDatastoreParams(fake.ServiceClient(), dsID, versionID).EachPage(func(page pagination.Page) (bool, error) {
		pages++

		actual, err := configurations.ExtractParams(page)
		if err != nil {
			return false, err
		}

		expected := []configurations.Param{
			{Max: 1, Min: 0, Name: "innodb_file_per_table", RestartRequired: true, Type: "integer"},
			{Max: 4294967296, Min: 0, Name: "key_buffer_size", RestartRequired: false, Type: "integer"},
			{Max: 65535, Min: 2, Name: "connect_timeout", RestartRequired: false, Type: "integer"},
			{Max: 4294967296, Min: 0, Name: "join_buffer_size", RestartRequired: false, Type: "integer"},
		}

		th.AssertDeepEquals(t, actual, expected)

		return true, nil
	})

	th.AssertNoErr(t, err)
	th.AssertEquals(t, 1, pages)
}

func TestGetDSParam(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()
	fixture.SetupHandler(t, dsParamGetURL, "GET", "", GetParamJSON, 200)

	param, err := configurations.GetDatastoreParam(fake.ServiceClient(), dsID, versionID, paramID).Extract()
	th.AssertNoErr(t, err)

	expected := &configurations.Param{
		Max: 1, Min: 0, Name: "innodb_file_per_table", RestartRequired: true, Type: "integer",
	}

	th.AssertDeepEquals(t, expected, param)
}

func TestListGlobalParams(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()
	fixture.SetupHandler(t, globalParamListURL, "GET", "", ListParamsJSON, 200)

	pages := 0
	err := configurations.ListGlobalParams(fake.ServiceClient(), versionID).EachPage(func(page pagination.Page) (bool, error) {
		pages++

		actual, err := configurations.ExtractParams(page)
		if err != nil {
			return false, err
		}

		expected := []configurations.Param{
			{Max: 1, Min: 0, Name: "innodb_file_per_table", RestartRequired: true, Type: "integer"},
			{Max: 4294967296, Min: 0, Name: "key_buffer_size", RestartRequired: false, Type: "integer"},
			{Max: 65535, Min: 2, Name: "connect_timeout", RestartRequired: false, Type: "integer"},
			{Max: 4294967296, Min: 0, Name: "join_buffer_size", RestartRequired: false, Type: "integer"},
		}

		th.AssertDeepEquals(t, actual, expected)

		return true, nil
	})

	th.AssertNoErr(t, err)
	th.AssertEquals(t, 1, pages)
}

func TestGetGlobalParam(t *testing.T) {
	th.SetupHTTP()
	defer th.TeardownHTTP()
	fixture.SetupHandler(t, globalParamGetURL, "GET", "", GetParamJSON, 200)

	param, err := configurations.GetGlobalParam(fake.ServiceClient(), versionID, paramID).Extract()
	th.AssertNoErr(t, err)

	expected := &configurations.Param{
		Max: 1, Min: 0, Name: "innodb_file_per_table", RestartRequired: true, Type: "integer",
	}

	th.AssertDeepEquals(t, expected, param)
}
