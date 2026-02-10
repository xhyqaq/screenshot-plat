package protocol

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

// 简单帧编解码自测，使用 net.Pipe
func TestLengthPrefixedFrame(t *testing.T) {
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	type payload struct {
		N int
		S string
	}
	want := payload{N: 42, S: "hello"}
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
