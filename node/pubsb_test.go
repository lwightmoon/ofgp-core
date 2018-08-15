package node

import "testing"

type testEvent struct {
}

func (e *testEvent) GetBusiness() string {
	return "p2p"
}
func (e *testEvent) GetEventType() uint8 {
	return 0
}
func (e *testEvent) GetFrom() uint8 {
	return 0
}
func (e *testEvent) GetTo() uint8 {
	return 1
}
func (e *testEvent) GetTxID() string {
	return "txid"
}
func (e *testEvent) GetData() []byte {
	return []byte("test event")
}
func TestPubSub(t *testing.T) {
	pubsub := newPubServer(1)
	topic := "topic1"
	ch := pubsub.subScribe(topic)
	pubsub.pub(topic, &WatchedEvent{
		business: "p2p",
		data:     &testEvent{},
		err:      nil,
	})
	ev := <-ch
	if val, ok := ev.(*WatchedEvent); ok {
		data := val.GetData()
		val2, _ := data.(PushEvent)
		t.Logf("%s,%s", val2.GetBusiness(), val2.GetData())
	}
}
