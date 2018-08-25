package p2p

func (tx *P2PTx) AddInfo(info *P2PInfo) {
	switch info.Msg.GetOperation() {
	case require:
		tx.Initiator = info
	case match:
		tx.Matcher = info
	default:
		p2pLogger.Error("add info err opration type not found")
	}
}

func (tx *P2PTx) SetFinished() {
	tx.Finished = true
}
func (info *P2PInfo) GetScTxID() string {
	event := info.Event
	if event != nil {
		return event.GetTxID()
	}
	p2pLogger.Error("event nil p2pInfo")
	return ""
}

func (info *P2PConfirmInfo) GetTxID() string {
	event := info.Event
	if event != nil {
		return event.GetTxID()
	}
	p2pLogger.Error("event nil p2pConfirmInfo")
	return ""
}

func (tx *P2PNewTx) SetFinished() {
	tx.Finished = true
}
