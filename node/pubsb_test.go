package node

import "testing"

type testEvent struct {
}

func (e *testEvent) GetBusiness() string {
	return "p2p"
}
func (e *testEvent) GetMethod() string {
	return "watched"
}
func (e *testEvent) GetFrom() string {
	return "bch"
}
func (e *testEvent) GetTo() string {
	return "eth"
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
	pubsub.pub(topic, &testEvent{})
	ev := <-ch
	t.Logf("%s", ev.GetData())
}
