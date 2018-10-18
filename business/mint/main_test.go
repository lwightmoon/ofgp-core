package mint

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/ofgp/ofgp-core/dgwdb"
)

var mintdb *mintDB

func TestMain(m *testing.M) {
	tmpDir, err := ioutil.TempDir("", "p2p")
	if err != nil {
		panic("create tempdir failed")
	}
	defer os.RemoveAll(tmpDir)
	db, _ := dgwdb.NewLDBDatabase(tmpDir, 1, 1)
	mintdb = newMintDB(db)
	code := m.Run()
	defer os.Exit(code)
}
