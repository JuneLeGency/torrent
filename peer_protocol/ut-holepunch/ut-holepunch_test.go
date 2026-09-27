package utHolepunch

import (
	"net/netip"
	"testing"

	qt "github.com/go-quicktest/qt"
)

var exampleMsgs = []Msg{
	{
		MsgType:  Rendezvous,
		AddrPort: netip.MustParseAddrPort("[1234::1]:42069"),
		ErrCode:  16777216,
	},
	{
		MsgType:  Connect,
		AddrPort: netip.MustParseAddrPort("1.2.3.4:42069"),
		ErrCode:  16777216,
	},
}

func TestUnmarshalMsg(t *testing.T) {
	for _, m := range exampleMsgs {
		b, err := m.MarshalBinary()
		qt.Assert(t, qt.IsNil(err))
		expectedLen := 24
		if m.AddrPort.Addr().Is4() {
			expectedLen = 12
		}
		qt.Check(t, qt.HasLen(b, expectedLen))
		var um Msg
		err = um.UnmarshalBinary(b)
		qt.Assert(t, qt.IsNil(err))
		qt.Check(t, qt.Equals(um, m))
	}
}

func TestUnmarshalAuroraShortNonErrorMessages(t *testing.T) {
	tests := []struct {
		payload []byte
		want    Msg
	}{
		{
			payload: []byte{0, 0, 1, 2, 3, 4, 0x1a, 0xe1},
			want:    Msg{MsgType: Rendezvous, AddrPort: netip.MustParseAddrPort("1.2.3.4:6881")},
		},
		{
			payload: []byte{1, 1, 0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0xc8, 0xd5},
			want:    Msg{MsgType: Connect, AddrPort: netip.MustParseAddrPort("[2001:db8::1]:51413")},
		},
	}
	for _, test := range tests {
		var got Msg
		qt.Assert(t, qt.IsNil(got.UnmarshalBinary(test.payload)))
		qt.Check(t, qt.Equals(got, test.want))
		standard, err := got.MarshalBinary()
		qt.Assert(t, qt.IsNil(err))
		qt.Check(t, qt.Equals(len(standard), len(test.payload)+4))
	}
}

func FuzzMsg(f *testing.F) {
	for _, m := range exampleMsgs {
		emb, err := m.MarshalBinary()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(emb)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		var m Msg
		err := m.UnmarshalBinary(b)
		if err != nil {
			t.SkipNow()
		}
		mb, err := m.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		var roundTrip Msg
		if err := roundTrip.UnmarshalBinary(mb); err != nil || roundTrip != m {
			t.FailNow()
		}
	})
}
