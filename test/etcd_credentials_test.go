// +build etcd

package test

import (
	"context"
	"testing"
)

// uses some stuff from etcd_tests.go

func TestEtcdCredentials(t *testing.T) {
	corefile := `.:0 {
    etcd skydns.test {
        path /skydns
    }
}`

	ex, _, _, err := CoreDNSServerAndPorts(corefile)
	if err != nil {
		t.Fatalf("Could not get CoreDNS serving instance: %s", err)
	}
	defer ex.Stop()

	etc := etcdPlugin()
	username := "root"
	password := "password"
	key := "foo"
	value := "bar"

	var ctx = context.TODO()

	if _, err := etc.Client.Put(ctx, key, value); err != nil {
		t.Errorf("Failed to put dummy value un etcd: %v", err)
	}

	if _, err := etc.Client.RoleAdd(ctx, "root"); err != nil {
		t.Errorf("Failed to create root role: %s", err)
	}
	if _, err := etc.Client.UserAdd(ctx, username, password); err != nil {
		t.Errorf("Failed to create user: %s", err)
	}
	if _, err := etc.Client.UserGrantRole(ctx, username, "root"); err != nil {
		t.Errorf("Failed to assign role to root user: %v", err)
	}
	if _, err := etc.Client.AuthEnable(ctx); err != nil {
		t.Errorf("Failed to enable authentication: %s", err)
	}

	etc2 := etcdPluginWithCredentials(username, password)

	defer func() {
		if _, err := etc2.Client.AuthDisable(ctx); err != nil {
			t.Errorf("Fail to disable authentication: %v", err)
		}
	}()

	resp, err := etc2.Client.Get(ctx, key)
	if err != nil {
		t.Errorf("Fail to retrieve value from etcd: %v", err)
	}

	if len(resp.Kvs) != 1 {
		t.Errorf("Too many response found: %+v", resp)
		return
	}
	actual := resp.Kvs[0].Value
	expected := "bar"
	if string(resp.Kvs[0].Value) != expected {
		t.Errorf("Value doesn't match, expected:%s actual:%s", actual, expected)
	}
}
