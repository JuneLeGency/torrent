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

// Removing a WebSeed while its request is still unwinding must not break the
// next request update, and a replacement WebSeed must still be scheduled.
func TestRemoveWebSeedsWithActiveRequestThenReplace(t *testing.T) {
	payload := make([]byte, 256*1024+3)
	rand.New(rand.NewSource(2)).Read(payload)
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
	// A refused connection makes the request fail at once; upstream then keeps
	// the cancelled request registered while it backs off before cleanup.
	refused := httptest.NewServer(http.NotFoundHandler())
	refusedURL := refused.URL + "/payload.bin"
	refused.Close()
	var served atomic.Int64
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served.Add(1)
		http.ServeContent(w, r, "payload.bin", time.Time{}, bytes.NewReader(payload))
	}))
	defer good.Close()

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
	tt.AddWebSeeds([]string{refusedURL})
	time.Sleep(300 * time.Millisecond)
	if n := tt.RemoveWebSeeds([]string{refusedURL}); n != 1 {
		t.Fatalf("removed=%d", n)
	}
	// Immediately schedule a replacement while the cancelled request unwinds.
	tt.AddWebSeeds([]string{good.URL + "/payload.bin"})
	select {
	case <-tt.Complete().On():
	case <-time.After(15 * time.Second):
		t.Fatalf("replacement did not complete: %d/%d served=%d", tt.BytesCompleted(), tt.Length(), served.Load())
	}
}
