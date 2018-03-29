package accounts_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/vechain/thor/api/accounts"
	"github.com/vechain/thor/block"
	"github.com/vechain/thor/chain"
	"github.com/vechain/thor/genesis"
	"github.com/vechain/thor/lvldb"
	"github.com/vechain/thor/state"
	"github.com/vechain/thor/thor"
)

const (
	emptyRootHash = "56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421"
	testAddress   = "56e81f171bcc55a6ff8345e692c0f86e5b48e01a"
)

type account struct {
	addr    thor.Address
	balance *big.Int
	code    []byte
	storage thor.Hash
}

var b, _ = new(big.Int).SetString("10000000000000000000000", 10)
var accs = []struct {
	in, want account
}{
	{
		account{thor.BytesToAddress([]byte("acc1")), b, []byte{0x11, 0x12}, thor.BytesToHash([]byte("v1"))},
		account{thor.BytesToAddress([]byte("acc1")), b, []byte{0x11, 0x12}, thor.BytesToHash([]byte("v1"))},
	},
	{
		account{thor.BytesToAddress([]byte("acc2")), big.NewInt(100), []byte{0x14, 0x15}, thor.BytesToHash([]byte("v2"))},
		account{thor.BytesToAddress([]byte("acc2")), big.NewInt(100), []byte{0x14, 0x15}, thor.BytesToHash([]byte("v2"))},
	},
	{
		account{thor.BytesToAddress([]byte("acc3")), big.NewInt(1000), []byte{0x20, 0x21}, thor.BytesToHash([]byte("v3"))},
		account{thor.BytesToAddress([]byte("acc3")), big.NewInt(1000), []byte{0x20, 0x21}, thor.BytesToHash([]byte("v3"))},
	},
}
var storageKey = thor.BytesToHash([]byte("key"))

func TestAccount(t *testing.T) {
	ts := initAccountServer(t)
	defer ts.Close()

	for _, v := range accs {
		address := v.in.addr
		r, err := httpGet(ts, ts.URL+fmt.Sprintf("/accounts/%v/balance", address.String()))
		if err != nil {
			t.Fatal(err)
		}
		var bal math.HexOrDecimal256
		if err := json.Unmarshal(r, &bal); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, math.HexOrDecimal256(*v.want.balance), bal, "balance should be equal")

		r, err = httpGet(ts, ts.URL+fmt.Sprintf("/accounts/%v/code", address.String()))
		if err != nil {
			t.Fatal(err)
		}
		var c string
		if err := json.Unmarshal(r, &c); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, hexutil.Encode(v.want.code), c, "code should be equal")

		r, err = httpGet(ts, ts.URL+fmt.Sprintf("/accounts/%v/storage?key=%v", address.String(), storageKey.String()))
		if err != nil {
			t.Fatal(err)
		}

		var value string
		if err := json.Unmarshal(r, &value); err != nil {
			t.Fatal(err)
		}
		h, err := thor.ParseHash(value)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, v.want.storage, h, "storage should be equal")

	}

}

func initAccountServer(t *testing.T) *httptest.Server {
	db, _ := lvldb.NewMem()
	hash, _ := thor.ParseHash(emptyRootHash)
	stateC := state.NewCreator(db)
	s, _ := stateC.NewState(hash)
	for _, v := range accs {
		address := v.in.addr
		s.SetBalance(address, v.in.balance)
		s.SetCode(address, v.in.code)
		s.SetStorage(address, storageKey, v.in.storage)
	}
	stateRoot, _ := s.Stage().Commit()
	chain := chain.New(db)
	b, _, err := genesis.Dev.Build(stateC)
	if err != nil {
		t.Fatal(err)
	}
	chain.WriteGenesis(b)
	best, _ := chain.GetBestBlock()
	bl := new(block.Builder).
		ParentID(best.Header().ID()).
		StateRoot(stateRoot).
		Build()
	if _, err := chain.AddBlock(bl, true); err != nil {
		t.Fatal(err)
	}
	router := mux.NewRouter()
	accounts.New(chain, stateC).Mount(router, "/accounts/")
	ts := httptest.NewServer(router)
	return ts
}

func httpGet(ts *httptest.Server, url string) ([]byte, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	r, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	return r, nil
}