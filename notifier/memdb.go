package notifier

import (
	"github.com/yunwilliamyu/contact-trace-mixnet/notifier/pb"
	"sync"
)

type inMemoryDrop struct {
	messages []*pb.Notification
}

type InMemoryDB struct {
	db map[DeadDropID]*inMemoryDrop
	mu sync.Mutex
}

func (memdb *InMemoryDB) getDrop(deaddropID DeadDropID) *inMemoryDrop {
	if memdb.db == nil {
		memdb.db = make(map[DeadDropID]*inMemoryDrop)
	}
	if d, ok := memdb.db[deaddropID]; ok {
		return d
	}
	d := &inMemoryDrop{}
	memdb.db[deaddropID] = d
	return d
}

func (memdb *InMemoryDB) Put(deaddropID DeadDropID, message *pb.Notification) error {
	memdb.mu.Lock()
	d := memdb.getDrop(deaddropID)
	d.messages = append(d.messages, message)
	memdb.mu.Unlock()
	return nil
}

func (memdb *InMemoryDB) Fetch(deaddropID DeadDropID, handler func(messages []*pb.Notification) (dropPrefix int, err error)) error {
	memdb.mu.Lock()
	defer memdb.mu.Unlock()

	d := memdb.getDrop(deaddropID)
	dropN, err := handler(d.messages)
	if err != nil {
		return err
	}
	d.messages = d.messages[dropN:]
	return nil
}
