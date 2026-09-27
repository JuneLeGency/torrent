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

// A WebSeed added to a torrent that already wants data must be scheduled
// without waiting for the periodic webseed request timer.
func TestAddWebSeedsSchedulesRequestsImmediately(t *testing.T) {
	payload := make([]byte, 300*1024+7)
	rand.New(rand.NewSource(1)).Read(payload)
	sourceDir := t.TempDir()
	sourcePath := filepath.Join(sourceDir, "payload.bin")
	if err := os.WriteFile(sourcePath, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	info := metainfo.Info{PieceLength: 16 * 1024}
	if err := info.BuildFromFilePath(sourcePath); err != nil {
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
	tt.DownloadAll()
	// Let the client's initial webseed timer tick pass so only the periodic
	// interval could otherwise schedule the new webseed.
	time.Sleep(100 * time.Millisecond)
	tt.AddWebSeeds([]string{server.URL + "/payload.bin"})
	deadline := time.Now().Add(webseedRequestUpdateTimerInterval / 2)
	for requests.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if requests.Load() == 0 {
		t.Fatalf("no webseed request within %v of AddWebSeeds", webseedRequestUpdateTimerInterval/2)
	}
	select {
	case <-tt.Complete().On():
	case <-time.After(10 * time.Second):
		t.Fatalf("incomplete: %d/%d bytes, requests=%d", tt.BytesCompleted(), tt.Length(), requests.Load())
	}
}
