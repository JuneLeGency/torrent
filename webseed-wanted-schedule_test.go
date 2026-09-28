package torrent

import (
	"bytes"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
)

// Magnet/spec WebSeeds are often known before any piece is wanted. When the
// caller later wants pieces, the WebSeed must be scheduled without waiting
// for the periodic webseed request timer.
func TestWebSeedScheduledWhenPiecesBecomeWanted(t *testing.T) {
	payload := make([]byte, 300*1024+11)
	rand.New(rand.NewSource(4)).Read(payload)
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	info := metainfo.Info{PieceLength: 16 * 1024}
	if err := info.BuildFromFilePath(path); err != nil {
		t.Fatal(err)
	}
	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.ServeContent(w, r, "payload.bin", time.Time{}, bytes.NewReader(payload))
	}))
	defer server.Close()
	cl, err := NewClient(TestingConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	tt, err := cl.AddTorrent(&metainfo.MetaInfo{InfoBytes: infoBytes})
	if err != nil {
		t.Fatal(err)
	}
	tt.AddWebSeeds([]string{server.URL + "/payload.bin"})
	// Let the add-time update run while nothing is wanted yet.
	time.Sleep(100 * time.Millisecond)
	if requests.Load() != 0 {
		t.Fatal("WebSeed requested before any piece was wanted")
	}
	tt.DownloadAll()
	deadline := time.Now().Add(webseedRequestUpdateTimerInterval / 2)
	for requests.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if requests.Load() == 0 {
		t.Fatalf("no WebSeed request within %v of wanting pieces", webseedRequestUpdateTimerInterval/2)
	}
	select {
	case <-tt.Complete().On():
	case <-time.After(10 * time.Second):
		t.Fatalf("incomplete: %d/%d", tt.BytesCompleted(), tt.Length())
	}
}
