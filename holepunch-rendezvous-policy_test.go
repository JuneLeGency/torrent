package torrent

import (
	"net/netip"
	"testing"

	g "github.com/anacrolix/generics"
	"github.com/anacrolix/torrent/metainfo"
	pp "github.com/anacrolix/torrent/peer_protocol"
	utHolepunch "github.com/anacrolix/torrent/peer_protocol/ut-holepunch"
)

func TestHolepunchRendezvousPolicyDefaultsAndLimit(t *testing.T) {
	for _, test := range []struct {
		name        string
		limit       int
		connections int
		want        int
	}{
		{name: "default-unlimited", connections: 5, want: 5},
		{name: "negative-unlimited", limit: -1, connections: 5, want: 5},
		{name: "limited", limit: 3, connections: 5, want: 3},
		{name: "limit-above-candidates", limit: 8, connections: 5, want: 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &Client{config: &ClientConfig{MaxHolepunchRendezvousRelays: test.limit}}
			torrent := &Torrent{cl: client, conns: make(map[*PeerConn]struct{})}
			for range test.connections {
				torrent.conns[policyTestPeerConn(torrent)] = struct{}{}
			}
			got := torrent.holepunchRendezvousRelays(netip.MustParseAddrPort("192.0.2.1:6881"))
			if len(got) != test.want {
				t.Fatalf("relays=%d want %d", len(got), test.want)
			}
		})
	}
}

func TestHolepunchRendezvousPolicyCanDenyAttempt(t *testing.T) {
	target := netip.MustParseAddrPort("192.0.2.2:6881")
	called := 0
	client := &Client{config: &ClientConfig{
		AllowHolepunchRendezvous: func(infoHash metainfo.Hash, gotTarget netip.AddrPort) bool {
			called++
			if infoHash != (metainfo.Hash{}) || gotTarget != target {
				t.Fatalf("policy input infoHash=%x target=%v", infoHash, gotTarget)
			}
			return false
		},
	}}
	torrent := &Torrent{cl: client, conns: map[*PeerConn]struct{}{}, infoHash: g.Some(metainfo.Hash{})}
	pc := policyTestPeerConn(torrent)
	torrent.conns[pc] = struct{}{}
	err := torrent.trySendHolepunchRendezvous(target)
	if err == nil || err.Error() != "holepunch rendezvous denied by policy" {
		t.Fatalf("deny error=%v", err)
	}
	if called != 1 || pc.messageWriter.writeBuffer.Len() != 0 {
		t.Fatalf("called=%d buffered=%d", called, pc.messageWriter.writeBuffer.Len())
	}
}

func policyTestPeerConn(torrent *Torrent) *PeerConn {
	return &PeerConn{
		Peer: Peer{t: torrent},
		PeerExtensionIDs: map[pp.ExtensionName]pp.ExtensionNumber{
			utHolepunch.ExtensionName: 1,
		},
		messageWriter: peerConnMsgWriter{
			writeBuffer: new(peerConnMsgWriterBuffer),
		},
	}
}
