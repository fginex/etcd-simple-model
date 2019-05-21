package etcd

import (
	"fmt"
	"log"
	"os/exec"
	"testing"
	"time"
)

const (
	ConfigAppEnv  = ServerEnvironmentDev
	ConfigAppName = "TestConfigClient"
	ConfigStruct  = ConfigStructureEnvKeyAsRoot
)

type testConfigClient struct {
}

func (t *testConfigClient) ReloadConfigChange(entryKey string, newVal string, prevVal string) {
	log.Printf("<<<<< ReloadConfigChange entryKey:%s newVal:%s prevVal:%s >>>>\n", entryKey, newVal, prevVal)
}

// ------------------------------------------------------------------------------
func init() {
	var err error

	log.Println("Test.init()")

	// Add some keys for testing. Make sure etcdctl is in a dir defined by $ETCD.
	for i := 0; i < 10; i++ {
		_, err = exec.Command("bash", "-c", fmt.Sprintf("${ETCD_DIR}/etcdctl put %s/%s/TestKey%d TestValue%d", ConfigAppEnv, ConfigAppName, i, i)).Output()
		if err != nil {
			fmt.Println(err)
		}
	}
}

// ------------------------------------------------------------------------------
func TestConfigClient_ConnectSuccess(t *testing.T) {

	// If you execute this test with etcd initially not running the client will wait until it starts up. Once etcd is started
	// it will read the entries and display them.

	etcd, err := CreateNewConfigClient([]string{"http://localhost:2379"}, ConfigAppName, ConfigAppEnv, ConfigStruct)
	if err != nil {
		t.Fatalf("%s", err)
	}

	// Wait a few seconds for its blood to start flowing.
	time.Sleep(3 * time.Second)

	// Test shutting it down.
	etcd.Close()
}

func TestConfigClient_ReadSingleEntry(t *testing.T) {

	etcd, err := CreateNewConfigClient([]string{"http://localhost:2379"}, ConfigAppName, ConfigAppEnv, ConfigStruct)
	if err != nil {
		t.Fatalf("%s", err)
	}

	// Some should return the default value
	for i := 0; i < 12; i++ {
		key := fmt.Sprintf("TestKey%d", i)
		val, err := etcd.ReadEntry(key, "MyDefaultValue")
		if err != nil {
			t.Fatalf("%s", err)
		}
		log.Printf("key: %s = %s\n", key, val)
	}
}

func TestConfigClient_ReadEntries(t *testing.T) {

	etcd, err := CreateNewConfigClient([]string{"http://localhost:2379"}, ConfigAppName, ConfigAppEnv, ConfigStruct)
	if err != nil {
		t.Fatalf("%s", err)
	}

	entries, err := etcd.ReadEntries("")
	if err != nil {
		t.Fatalf("%s", err)
	}

	log.Println(entries)
}

func TestConfigClient_EntryChangeWithNoPrefix(t *testing.T) {

	etcd, err := CreateNewConfigClient([]string{"http://localhost:2379"}, ConfigAppName, ConfigAppEnv, ConfigStruct)
	if err != nil {
		t.Fatalf("%s", err)
	}

	testKey := "TestKey2"

	err = etcd.AddStringKvHandler(testKey, false, &testConfigClient{})
	if err != nil {
		t.Fatalf("%s", err)
	}

	// Change the value of a test key
	_, err = exec.Command("bash", "-c", fmt.Sprintf("${ETCD_DIR}/etcdctl put %s/%s/%s SomeNewTestValue!", ConfigAppEnv, ConfigAppName, testKey)).Output()

	time.Sleep(3 * time.Second)
}

func TestConfigClient_EntryChangeWithPrefix(t *testing.T) {

	etcd, err := CreateNewConfigClient([]string{"http://localhost:2379"}, ConfigAppName, ConfigAppEnv, ConfigStruct)
	if err != nil {
		t.Fatalf("%s", err)
	}

	err = etcd.AddStringKvHandler("", true, &testConfigClient{})
	if err != nil {
		t.Fatalf("%s", err)
	}

	// Change the value of a test key
	_, err = exec.Command("bash", "-c", fmt.Sprintf("${ETCD_DIR}/etcdctl put %s/%s/%s SomeNewTestValue!", ConfigAppEnv, ConfigAppName, "TestKey5")).Output()

	time.Sleep(3 * time.Second)
}

func TestConfigClient_ConnectFail(t *testing.T) {

	// TODO: This will eventually timeout (after 30 seconds) via the go test but the implementing apps will need a way to handle this better. 

	etcd, err := CreateNewConfigClient([]string{"http://NotResolvable:2379"}, ConfigAppName, ConfigAppEnv, ConfigStruct)
	if err != nil {
		t.Fatalf("%s", err)
	}

	log.Println("Trying to read entries... (blocks)")
	entries, err := etcd.ReadEntries("")
	if err != nil {
		t.Fatalf("%s", err)
	}

	log.Println(entries)
}
