package protocol

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestLengthPrefixedFrame(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	type payload struct {
		A int
		B string
	}
	want := payload{A: 7, B: "x"}
	b, _ := json.Marshal(want)

	go func() {
		if err := SendWithLengthPrefix(c1, b); err != nil {
			t.Errorf("send: %v", err)
		}
	}()

	c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	got, err := ReadWithLengthPrefix(c2)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != string(b) {
		t.Fatalf("mismatch: %q != %q", string(got), string(b))
	}
}
