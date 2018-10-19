package mint

import (
	"bytes"
	"encoding/binary"

	"github.com/ofgp/ofgp-core/business"
)

func (mintReq *MintRequire) decode(data []byte) {
	r := bytes.NewReader(data)
	business.ReadInt(r, &mintReq.TokenFrom)
	business.ReadInt(r, &mintReq.TokenTo)
	receiver := business.Read(r, 25)
	mintReq.Receiver = receiver
}

func (mintReq *MintRequire) encode() []byte {
	buf := &bytes.Buffer{}
	binary.Write(buf, binary.BigEndian, mintReq.TokenFrom)
	binary.Write(buf, binary.BigEndian, mintReq.TokenTo)
	binary.Write(buf, binary.BigEndian, mintReq.Receiver)
	return buf.Bytes()
}
