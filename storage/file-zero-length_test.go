package storage

import (
	"bytes"
	"testing"

	"github.com/anacrolix/torrent/metainfo"
)

// A piece spanning a zero-length file must be writable with the default
// file IO (mmap). Mapping a zero-byte file fails with EINVAL.
func TestFileStorageWritesPieceSpanningZeroLengthFile(t *testing.T) {
	const pieceLength = 16 * 1024
	info := &metainfo.Info{
		Name:        "zero",
		PieceLength: pieceLength,
		Files: []metainfo.FileInfo{
			{Path: []string{"a.bin"}, Length: 10 * 1024},
			{Path: []string{"empty.trigger"}, Length: 0},
			{Path: []string{"b.bin"}, Length: 10 * 1024},
		},
	}
	info.Pieces = make([]byte, 20*2) // two pieces; hashes are irrelevant for raw writes
	client := NewFile(t.TempDir())
	defer client.Close()
	tor, err := client.OpenTorrent(t.Context(), info, metainfo.Hash{})
	if err != nil {
		t.Fatal(err)
	}
	defer tor.Close()
	data := bytes.Repeat([]byte{0x5a}, pieceLength)
	piece := tor.Piece(info.Piece(0))
	if _, err := piece.WriteAt(data, 0); err != nil {
		t.Fatalf("write piece spanning zero-length file: %v", err)
	}
	got := make([]byte, pieceLength)
	if _, err := piece.ReadAt(got, 0); err != nil {
		t.Fatalf("read piece spanning zero-length file: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("piece bytes changed across a zero-length file")
	}
}
