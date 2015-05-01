package integrationtest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	random "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-random"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	"github.com/ipfs/go-ipfs/core"
	coreunix "github.com/ipfs/go-ipfs/core/coreunix"
	mn2 "github.com/ipfs/go-ipfs/p2p/net/mock2"
	"github.com/ipfs/go-ipfs/p2p/peer"
	"github.com/ipfs/go-ipfs/thirdparty/unit"
	testutil "github.com/ipfs/go-ipfs/util/testutil"
)

const kSeed = 1

func Test1KBInstantaneous(t *testing.T) {
	conf := testutil.LatencyConfig{
		NetworkLatency:    0,
		RoutingLatency:    0,
		BlockstoreLatency: 0,
	}

	if err := DirectAddCat(RandomBytes(1*unit.KB), conf); err != nil {
		t.Fatal(err)
	}
}

func TestDegenerateSlowBlockstore(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{BlockstoreLatency: 50 * time.Millisecond}
	if err := AddCatPowers(conf, 128); err != nil {
		t.Fatal(err)
	}
}

func TestDegenerateSlowNetwork(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{NetworkLatency: 400 * time.Millisecond}
	if err := AddCatPowers(conf, 128); err != nil {
		t.Fatal(err)
	}
}

func TestDegenerateSlowRouting(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{RoutingLatency: 400 * time.Millisecond}
	if err := AddCatPowers(conf, 128); err != nil {
		t.Fatal(err)
	}
}

func Test100MBMacbookCoastToCoast(t *testing.T) {
	SkipUnlessEpic(t)
	conf := testutil.LatencyConfig{}.NetworkNYtoSF().BlockstoreSlowSSD2014().RoutingSlow()
	if err := DirectAddCat(RandomBytes(100*1024*1024), conf); err != nil {
		t.Fatal(err)
	}
}

func AddCatPowers(conf testutil.LatencyConfig, megabytesMax int64) error {
	var i int64
	for i = 1; i < megabytesMax; i = i * 2 {
		fmt.Printf("%d MB\n", i)
		if err := DirectAddCat(RandomBytes(i*1024*1024), conf); err != nil {
			return err
		}
	}
	return nil
}

func RandomBytes(n int64) []byte {
	data := new(bytes.Buffer)
	random.WritePseudoRandomBytes(n, data, kSeed)
	return data.Bytes()
}

func DirectAddCat(data []byte, conf testutil.LatencyConfig) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const numPeers = 2

	// create network
	ns, err := mn2.NewNetworkSimulator(numPeers)
	if err != nil {
		return err
	}
	defer ns.Close()
	ns.ConOpts = mn2.ConnectionOpts{
		Bandwidth: 500 * unit.MB,
		Delay:     conf.NetworkLatency,
	}

	peers := ns.Peers()
	if len(peers) < numPeers {
		return errors.New("test initialization error")
	}

	adder, err := core.NewIPFSNode(ctx, MocknetTestRepo(peers[0], ns, conf, core.DHTOption))
	if err != nil {
		return err
	}
	defer adder.Close()
	catter, err := core.NewIPFSNode(ctx, MocknetTestRepo(peers[1], ns, conf, core.DHTOption))
	if err != nil {
		return err
	}
	defer catter.Close()

	bs1 := []peer.PeerInfo{adder.Peerstore.PeerInfo(adder.Identity)}
	bs2 := []peer.PeerInfo{catter.Peerstore.PeerInfo(catter.Identity)}

	if err := catter.Bootstrap(core.BootstrapConfigWithPeers(bs1)); err != nil {
		return err
	}
	if err := adder.Bootstrap(core.BootstrapConfigWithPeers(bs2)); err != nil {
		return err
	}

	added, err := coreunix.Add(adder, bytes.NewReader(data))
	if err != nil {
		return err
	}

	readerCatted, err := coreunix.Cat(catter, added)
	if err != nil {
		return err
	}

	// verify
	bufout := new(bytes.Buffer)
	io.Copy(bufout, readerCatted)
	if 0 != bytes.Compare(bufout.Bytes(), data) {
		return errors.New("catted data does not match added data")
	}
	return nil
}

func SkipUnlessEpic(t *testing.T) {
	if os.Getenv("IPFS_EPIC_TEST") == "" {
		t.SkipNow()
	}
}
