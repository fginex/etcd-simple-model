package etcd

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"go.etcd.io/etcd/clientv3"
)

// -----------------------------------------------------------------------------------
// ConfigClient for etcd
// -----------------------------------------------------------------------------------
//
// etcd config structure can either be this:
//
// "app-key/env/entry-key"
//
// APP-KEY-1
//    +
//    + + + PROD
//    +      + + + Entry-Key1
//    +      + + + Entry-Key2
//    +      + + + Entry-Key(n)...
//    +
//    + + + STAGE
//    +      + + + Entry-Key1
//    +      + + + Entry-Key2
//    +      + + + Entry-Key(n)...
//    +
//    + + + QA
//           + + + Entry-Key1
//           + + + Entry-Key2
//           + + + Entry-Key(n)...
//
//
// Or this:
//
// "env/app-key/entry-key"
//
//    PROD
//     +   +
//     +   + + APP-KEY-1
//     +      + + Entry-Key1
//     +      + + Entry-Key2
//     +      + + Entry-Key(n)...
//     +
//    STAGE
//     +   +
//     +   + APP-KEY-1
//     +      + + Entry-Key1
//     +      + + Entry-Key2
//     +      + + Entry-Key(n)...
//     +
//    QA +++
//         + + APP-KEY-1
//            + + Entry-Key1
//            + + Entry-Key2
//            + + Entry-Key(n)...
//
//
// Or this:
//
//   "app-key/entry-key" (if ServerEnvironmentNone is specified)
//
//
// -----------------------------------------------------------------------------------

const (
	etcdServiceDialTimeout = 5 * time.Second
)

// ConfigStructureType specifies how the config entries are stored in etcd. Either with app-key as the root or environment.
type ConfigStructureType int

const (
	// ConfigStructureAppKeyAsRoot - etcd config structure is as follows: "app-key/env/entry-key" or "app-key/entry-key" if ServerEnvironmentNone is specified.
	ConfigStructureAppKeyAsRoot = ConfigStructureType(iota)

	// ConfigStructureEnvKeyAsRoot - etcd config structure is as follows: "env/app-key/entry-key"
	ConfigStructureEnvKeyAsRoot
)

// ServerEnvironmentType - used to indicate qa, stage or production environments
type ServerEnvironmentType int

func (s ServerEnvironmentType) String() string {
	switch s {
	case ServerEnvironmentDev:
		return "Dev"
	case ServerEnvironmentQA:
		return "Qa"
	case ServerEnvironmentStage:
		return "Stage"
	case ServerEnvironmentProduction:
		return "Prod"
	case ServerEnvironmentNone:
		return ""
	default:
		return "Unknown"
	}
}

const (
	// ServerEnvironmentDev - indicates dev environment
	ServerEnvironmentDev = ServerEnvironmentType(iota)

	// ServerEnvironmentQA - indicates qa environment
	ServerEnvironmentQA

	// ServerEnvironmentStage - indicates stage environment
	ServerEnvironmentStage

	// ServerEnvironmentProduction - indicates production environment
	ServerEnvironmentProduction

	// ServerEnvironmentNone - indicates no environment level in config structure
	ServerEnvironmentNone
)

// ConfigChangeHandler interface for config change handlers.
type ConfigChangeHandler interface {
	ReloadConfigChange(entryKey string, newVal string, prevVal string)
}

type watcherInfo struct {
	handler ConfigChangeHandler
	key     string
	ch      clientv3.WatchChan
	prefix  bool
}

// ConfigClient observes config changes on etcd and executes app specific code associated these changes. It also allows the app to retrieve entries on demand.
type ConfigClient struct {
	Endpoints []string
	AppKey    string
	Env       ServerEnvironmentType
	Org       ConfigStructureType

	shutdown        chan struct{}
	shutdownInvoked bool
	wg              sync.WaitGroup

	etcd       *clientv3.Client
	closeMutex sync.Mutex

	addCh    chan *watcherInfo
	removeCh chan string
}

// CreateNewConfigClient creates a new ConfigWatcher.
func CreateNewConfigClient(endpoints []string, appKey string, env ServerEnvironmentType, org ConfigStructureType) (*ConfigClient, error) {
	var err error

	w := ConfigClient{
		Endpoints:       endpoints,
		AppKey:          appKey,
		Env:             env,
		Org:             org,
		shutdown:        make(chan struct{}),
		shutdownInvoked: false,
		addCh:           make(chan *watcherInfo),
		removeCh:        make(chan string),
	}

	w.etcd, err = clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: etcdServiceDialTimeout,
	})
	if err != nil {
		return nil, err
	}

	conn := w.etcd.ActiveConnection()
	if conn == nil {
		return nil, errors.New("no active connection")
	}

	conn.GetState()

	go w.runLoop()

	return &w, nil
}

func (cc *ConfigClient) runLoop() {
	cc.wg.Add(1)
	defer cc.wg.Done()

	var channelList []*watcherInfo
	var cases []reflect.SelectCase

	etcdWatchers := map[string]*watcherInfo{}

	for {
		select {
		case <-cc.shutdown:
			// Clear out the map and exit
			for k := range etcdWatchers {
				delete(etcdWatchers, k)
			}
			return

		case key := <-cc.removeCh:
			delete(etcdWatchers, key)

		case info := <-cc.addCh:
			if info.prefix {
				info.ch = cc.etcd.Watch(context.Background(), cc.expandKey(info.key), clientv3.WithPrevKV(), clientv3.WithPrefix())
			} else {
				info.ch = cc.etcd.Watch(context.Background(), cc.expandKey(info.key), clientv3.WithPrevKV())
			}
			etcdWatchers[info.key] = info

			cases = []reflect.SelectCase{}
			channelList = []*watcherInfo{}

			for _, w := range etcdWatchers {
				cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(w.ch)})
				channelList = append(channelList, w)
			}

		default:
			remaining := len(cases)
			for remaining > 0 {
				selectedIndex, val, ok := reflect.Select(cases)
				if !ok {
					// response channel closed - remove it
					delete(etcdWatchers, channelList[selectedIndex].key)
				} else {
					switch v := val.Interface().(type) {
					case clientv3.WatchResponse:

						for _, evt := range v.Events {
							prevKv := ""

							if evt.PrevKv != nil {
								prevKv = string(evt.PrevKv.Value)
							}

							channelList[selectedIndex].handler.ReloadConfigChange(
								string(evt.Kv.Key), string(evt.Kv.Value), prevKv,
							)
						}

					default:
						panic(fmt.Sprintf("unexpected type %T received from etcd response channel", v))
					}
				}
				remaining--
			}
		}
	}
}

// AddStringKvHandler adds a new config handler method to be executed when the specified entry changes. Handles all keys and values as strings.
func (cc *ConfigClient) AddStringKvHandler(key string, prefix bool, handler ConfigChangeHandler) error {
	if cc.shutdownInvoked {
		return errors.New("all connections have been closed via Close()")
	}

	cc.addCh <- &watcherInfo{handler: handler, key: key, prefix: prefix}
	return nil
}

// RemoveHandler removes all associated config change handlers for the specified key
func (cc *ConfigClient) RemoveHandler(key string) error {
	if cc.shutdownInvoked {
		return errors.New("all connections have been closed via Close()")
	}

	cc.removeCh <- key
	return nil
}

// ReadEntry reads a config entry from etcd
func (cc *ConfigClient) ReadEntry(entryKey string, defaultVal string) (string, error) {

	if cc.shutdownInvoked {
		return defaultVal, errors.New("all connections have been closed via Close()")
	}

	rsp, err := cc.etcd.Get(context.Background(), cc.expandKey(entryKey))
	if err != nil {
		return defaultVal, err
	}

	if len(rsp.Kvs) == 0 {
		return defaultVal, nil
	}

	return string(rsp.Kvs[0].Value), nil
}

// ReadEntries reads config entries from etcd based on a prefix
func (cc *ConfigClient) ReadEntries(entryKeyPrefix string) (map[string]string, error) {
	retMap := map[string]string{}

	if cc.shutdownInvoked {
		return retMap, errors.New("all connections have been closed via Close()")
	}

	rsp, err := cc.etcd.Get(context.Background(), cc.expandKey(entryKeyPrefix), clientv3.WithPrefix())
	if err != nil {
		return retMap, err
	}

	for _, kv := range rsp.Kvs {
		retMap[string(kv.Key)] = string(kv.Value)
	}

	return retMap, nil
}

// Close closes all connections to etcd and shutdown any running go routines. Renders the instance of ConfigWatcher unusable.
func (cc *ConfigClient) Close() {
	cc.closeMutex.Lock()
	defer cc.closeMutex.Unlock()

	if cc.shutdownInvoked {
		return
	}

	cc.shutdownInvoked = true

	close(cc.shutdown)
	cc.wg.Wait()

	cc.etcd.Close()

	close(cc.addCh)
	close(cc.removeCh)
}

func (cc *ConfigClient) expandKey(key string) string {

	switch cc.Org {
	case ConfigStructureAppKeyAsRoot:
		switch cc.Env {
		case ServerEnvironmentProduction:
			fallthrough
		case ServerEnvironmentStage:
			fallthrough
		case ServerEnvironmentQA:
			fallthrough
		case ServerEnvironmentDev:
			return fmt.Sprintf("%s/%s/%s", cc.AppKey, cc.Env, key)
		case ServerEnvironmentNone:
			return fmt.Sprintf("%s/%s", cc.AppKey, key)
		default:
			panic(fmt.Sprintf("undefined ServerEnvironmentType %d", cc.Env))
		}

	case ConfigStructureEnvKeyAsRoot:
		switch cc.Env {
		case ServerEnvironmentProduction:
			fallthrough
		case ServerEnvironmentStage:
			fallthrough
		case ServerEnvironmentQA:
			fallthrough
		case ServerEnvironmentDev:
			return fmt.Sprintf("%s/%s/%s", cc.Env, cc.AppKey, key)
		case ServerEnvironmentNone:
			return fmt.Sprintf("%s/%s", cc.AppKey, key)
		default:
			panic(fmt.Sprintf("undefined ServerEnvironmentType %d", cc.Env))
		}

	default:
		panic(fmt.Sprintf("undefined ConfigStructureType %d", cc.Org))
	}
}
