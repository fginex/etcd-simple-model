package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/fginex/etcd-simple-model/etcd"
)

type myConfigReloader struct {
}

func (t *myConfigReloader) ReloadConfigChange(entryKey string, newVal string, prevVal string) {
	log.Printf("<<<<< ReloadConfigChange entryKey:%s newVal:%s prevVal:%s >>>>\n", entryKey, newVal, prevVal)
}

func main() {

	log.Println("Connecting to etcd... (Note this will wait until you start it)")

	etcd, err := etcd.CreateNewConfigClient(
		[]string{"http://localhost:2379"},
		"TestApp1",
		etcd.ServerEnvironmentNone,
		etcd.ConfigStructureAppKeyAsRoot)

	if err != nil {
		panic(fmt.Sprintf("%s", err))
	}

	// Watch a single key in this app's config. You can test this by running this: ./etcdctl put TestApp1/TestKey1 "This is test value for TestKey1"
	etcd.AddStringKvHandler("TestKey1", false, &myConfigReloader{})

	// Watch all keys in this app's config. You can test using the command in the above comment or
	// by running this: ./etcdctl put TestApp1/TestKey5 "This is test value for TestKey5"
	etcd.AddStringKvHandler("", true, &myConfigReloader{})

	log.Println("Waiting for config changes...")

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT)
	<-sig

}
