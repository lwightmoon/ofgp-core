package p2p

func (tx *P2PTx) AddInfo(info *P2PInfo) {
	switch info.Msg.GetOperation() {
	case require:
		tx.Initiator = info
	case match:
		tx.Matcher = info
	}
}

func (info *P2PInfo) GetScTxID() string {
	return info.Event.GetTxID()
}

func (info *P2PConfirmInfo) GetTxID() string {
	return info.Event.GetTxID()
}
