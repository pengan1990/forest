package forest

import (
	"context"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"go.etcd.io/etcd/clientv3"
	"log"
	"time"
)

type Etcd struct {
	endpoints []string
	client    *clientv3.Client
	kv        clientv3.KV

	timeout time.Duration
}

// create a etcd
func NewEtcd(endpoints []string, timeout time.Duration) (etcd *Etcd, err error) {

	var (
		client *clientv3.Client
	)

	conf := clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: timeout,
	}
	if client, err = clientv3.New(conf); err != nil {
		return
	}

	etcd = &Etcd{

		endpoints: endpoints,
		client:    client,
		kv:        clientv3.NewKV(client),
		timeout:   timeout,
	}

	return
}

// get value  from a key
func (etcd *Etcd) Get(key string) (value []byte, err error) {

	var (
		getResponse *clientv3.GetResponse
	)
	ctx, cancelFunc := context.WithTimeout(context.Background(), etcd.timeout)
	defer cancelFunc()

	if getResponse, err = etcd.kv.Get(ctx, key); err != nil {
		return
	}

	if len(getResponse.Kvs) == 0 {
		return
	}

	value = getResponse.Kvs[0].Value

	return

}

// get values from  prefixKey
func (etcd *Etcd) GetWithPrefixKey(prefixKey string) (keys [][]byte, values [][]byte, err error) {

	var (
		getResponse *clientv3.GetResponse
	)
	ctx, cancelFunc := context.WithTimeout(context.Background(), etcd.timeout)
	defer cancelFunc()

	if getResponse, err = etcd.kv.Get(ctx, prefixKey, clientv3.WithPrefix()); err != nil {
		return
	}

	if len(getResponse.Kvs) == 0 {
		return
	}

	keys = make([][]byte, len(getResponse.Kvs))
	values = make([][]byte, len(getResponse.Kvs))

	for i := 0; i < len(getResponse.Kvs); i++ {
		keys = append(keys, getResponse.Kvs[i].Key)
		values = append(values, getResponse.Kvs[i].Value)
	}

	return

}

// put a key
func (etcd *Etcd) Put(key, value string) (err error) {

	ctx, cancelFunc := context.WithTimeout(context.Background(), etcd.timeout)
	defer cancelFunc()

	if _, err = etcd.kv.Put(ctx, key, value); err != nil {
		return
	}

	return
}

// put a key not exist
func (etcd *Etcd) PutNotExist(key, value string) (success bool, oldValue []byte, err error) {

	var (
		txnResponse *clientv3.TxnResponse
	)
	ctx, cancelFunc := context.WithTimeout(context.Background(), etcd.timeout)
	defer cancelFunc()

	txn := etcd.client.Txn(ctx)

	txnResponse, err = txn.If(clientv3.Compare(clientv3.Version(key), "=", 0)).

		Then(clientv3.OpPut(key, value)).
		Else(clientv3.OpGet(key)).
		Commit()

	if err != nil {
		return
	}

	if txnResponse.Succeeded {
		success = true
	} else {
		oldValue = make([]byte, 0)
		oldValue = txnResponse.Responses[0].GetResponseRange().Kvs[0].Value
	}

	return
}

func (etcd *Etcd) Update(key, value, oldValue string) (success bool, err error) {

	var (
		txnResponse *clientv3.TxnResponse
	)

	ctx, cancelFunc := context.WithTimeout(context.Background(), etcd.timeout)
	defer cancelFunc()

	txn := etcd.client.Txn(ctx)

	txnResponse, err = txn.If(clientv3.Compare(clientv3.Value(key), "=", oldValue)).
		Then(clientv3.OpPut(key, value)).
		Commit()

	if err != nil {
		return
	}

	if txnResponse.Succeeded {
		success = true
	}

	return
}

func (etcd *Etcd) Delete(key string) (err error) {

	ctx, cancelFunc := context.WithTimeout(context.Background(), etcd.timeout)
	defer cancelFunc()

	_, err = etcd.kv.Delete(ctx, key)

	return
}

// delete the keys  with prefix key
func (etcd *Etcd) DeleteWithPrefixKey(prefixKey string) (err error) {

	ctx, cancelFunc := context.WithTimeout(context.Background(), etcd.timeout)
	defer cancelFunc()

	_, err = etcd.kv.Delete(ctx, prefixKey, clientv3.WithPrefix())

	return
}

// watch a key
func (etcd *Etcd) Watch(key string) (keyChangeEventResponse *WatchKeyChangeResponse) {

	watcher := clientv3.NewWatcher(etcd.client)

	watchChans := watcher.Watch(context.Background(), key)

	keyChangeEventResponse = &WatchKeyChangeResponse{
		Event:   make(chan *KeyChangeEvent, 250),
		watcher: watcher,
	}

	go func() {

		for ch := range watchChans {

			if ch.Canceled {
				goto End
			}
			for _, event := range ch.Events {
				etcd.handleKeyChangeEvent(event, keyChangeEventResponse.Event)
			}
		}

	End:
		log.Println("the watcher lose for key:", key)
	}()

	return
}

// watch with prefix key
func (etcd *Etcd) WatchWithPrefixKey(prefixKey string) (keyChangeEventResponse *WatchKeyChangeResponse) {

	watcher := clientv3.NewWatcher(etcd.client)

	watchChans := watcher.Watch(context.Background(), prefixKey, clientv3.WithPrefix())

	keyChangeEventResponse = &WatchKeyChangeResponse{
		Event:   make(chan *KeyChangeEvent, 250),
		watcher: watcher,
	}

	go func() {

		for ch := range watchChans {

			if ch.Canceled {
				goto End
			}
			for _, event := range ch.Events {
				etcd.handleKeyChangeEvent(event, keyChangeEventResponse.Event)
			}
		}

	End:
		log.Println("the watcher lose for prefixKey:", prefixKey)
	}()

	return
}

// handle the key change event
func (etcd *Etcd) handleKeyChangeEvent(event *clientv3.Event, events chan *KeyChangeEvent) {

	changeEvent := &KeyChangeEvent{
		Key: string(event.Kv.Key),
	}
	switch event.Type {

	case mvccpb.PUT:
		if event.IsCreate() {
			changeEvent.Type = KeyCreateChangeEvent
		} else {
			changeEvent.Type = KeyCreateChangeEvent
		}
		changeEvent.Value = event.Kv.Value
	case mvccpb.DELETE:

		changeEvent.Type = KeyDeleteChangeEvent
	}
	events <- changeEvent

}
