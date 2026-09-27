package torrent

import (
	"testing"

	"github.com/anacrolix/torrent/metainfo"
)

func TestRemoveWebSeedsIsIdempotentAndAllowsReAdd(t *testing.T) {
	cl, err := NewClient(TestingConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	tt, _ := cl.AddTorrentInfoHash(metainfo.Hash{1})
	one := "https://one.example.invalid/file"
	two := "https://two.example.invalid/file"
	tt.AddWebSeeds([]string{one, two, one})
	oldPeers := tt.WebseedPeerConns()
	if len(oldPeers) != 2 {
		t.Fatalf("webseed peers=%d want 2", len(oldPeers))
	}

	removed := tt.RemoveWebSeeds([]string{
		"https://missing.example.invalid/file", one, two, one,
	})
	if removed != 2 || len(tt.WebseedPeerConns()) != 0 {
		t.Fatalf("removed=%d remaining=%d", removed, len(tt.WebseedPeerConns()))
	}
	for index, peer := range oldPeers {
		if !peer.closed.IsSet() {
			t.Fatalf("removed peer %d is not closed", index)
		}
	}
	if removed = tt.RemoveWebSeeds([]string{one, two}); removed != 0 {
		t.Fatalf("idempotent removal=%d want 0", removed)
	}

	tt.AddWebSeeds([]string{one})
	newPeers := tt.WebseedPeerConns()
	if len(newPeers) != 1 || newPeers[0].closed.IsSet() {
		t.Fatalf("re-added peers=%d closed=%t", len(newPeers), len(newPeers) == 1 && newPeers[0].closed.IsSet())
	}
}
