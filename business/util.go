package business

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/dgwdb"
	"github.com/ofgp/ofgp-core/util"
)

var bigEndian = binary.BigEndian

// Read 读取定长[]byte
func Read(r io.Reader, length int) []byte {
	res := make([]byte, length)
	io.ReadFull(r, res)
	return res
}

// ReadInt 读取int
func ReadInt(r io.Reader, element interface{}) error {
	var err error
	switch e := element.(type) {
	case *uint8:
		buf := make([]byte, 1)
		_, err = io.ReadFull(r, buf)
		*e = buf[0]
	case *uint32:
		buf := make([]byte, 4)
		_, err = io.ReadFull(r, buf)
		*e = bigEndian.Uint32(buf)
	case *uint64:
		buf := make([]byte, 8)
		_, err = io.ReadFull(r, buf)
		*e = bigEndian.Uint64(buf)
	}
	return err
}

// OpenDbOrDie create ldb
func OpenDbOrDie(dbPath, subPath string) (db *dgwdb.LDBDatabase, newlyCreated bool) {
	if len(dbPath) == 0 {
		homeDir, err := util.GetHomeDir()
		if err != nil {
			panic("Cannot detect the home dir for the current user.")
		}
		dbPath = path.Join(homeDir, subPath)
	}

	fmt.Println("open db path ", dbPath)
	info, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		if err := os.Mkdir(dbPath, 0700); err != nil {
			panic(fmt.Errorf("Cannot create db path %v,err:%v", dbPath, err))
		}
		newlyCreated = true
	} else {
		if err != nil {
			panic(fmt.Errorf("Cannot get info of %v", dbPath))
		}
		if !info.IsDir() {
			panic(fmt.Errorf("Datavse path (%v) is not a directory", dbPath))
		}
		if c, _ := ioutil.ReadDir(dbPath); len(c) == 0 {
			newlyCreated = true
		} else {
			newlyCreated = false
		}
	}

	db, err = dgwdb.NewLDBDatabase(dbPath, cluster.DbCache, cluster.DbFileHandles)
	if err != nil {
		panic(fmt.Errorf("Failed to open database at %v", dbPath))
	}
	return
}
