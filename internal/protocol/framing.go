package protocol

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

// MaxMessageBytes caps a single framed message.
//
// Without a cap, one peer could make the other allocate without bound, which matters
// because the daemon accepts connections from any process running as this user. The
// value is generous: the largest real message is a capability snapshot or a form
// description, both well under a megabyte.
const MaxMessageBytes = 1 << 20

// Writer frames messages for a stream. Messages are newline-delimited: JSON escapes
// any newline inside a string, so a message can never contain a bare newline and the
// framing needs no length prefix.
//
// Every message is flushed. A peer waiting on a request must not be blocked behind a
// buffer that only empties when more traffic happens to arrive.
type Writer struct {
	w *bufio.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{w: bufio.NewWriter(w)}
}

// WriteMessage encodes v as one line.
func (w *Writer) WriteMessage(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}
	if len(b) > MaxMessageBytes {
		return fmt.Errorf("message of %d bytes exceeds the %d byte cap", len(b), MaxMessageBytes)
	}
	if _, err := w.w.Write(b); err != nil {
		return err
	}
	if err := w.w.WriteByte('\n'); err != nil {
		return err
	}
	return w.w.Flush()
}

// Reader yields one validated message at a time from a stream.
type Reader struct {
	r *bufio.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{r: bufio.NewReader(r)}
}

// Read returns the next message, or io.EOF at a clean end of stream.
//
// Framing and validation are done together on purpose: a caller should never have to
// remember to call Decode as well, and a caller that forgot would accept a malformed
// message from a peer whose build disagrees with ours.
//
// A stream that ends mid-message is an error rather than EOF. A peer that died during
// a write leaves a partial line behind, and reporting that as a clean shutdown would
// turn a crash into a mystery.
func (r *Reader) Read() (Incoming, error) {
	line, err := r.readLine()
	if err != nil {
		return Incoming{}, err
	}
	return Decode(line)
}

func (r *Reader) readLine() ([]byte, error) {
	var buf []byte
	for {
		chunk, err := r.r.ReadSlice('\n')
		buf = append(buf, chunk...)
		if len(buf) > MaxMessageBytes {
			return nil, fmt.Errorf("message exceeds the %d byte cap", MaxMessageBytes)
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err == io.EOF {
			if len(buf) == 0 {
				return nil, io.EOF
			}
			return nil, fmt.Errorf("truncated message: %d bytes with no terminator", len(buf))
		}
		if err != nil {
			return nil, err
		}
		return buf[:len(buf)-1], nil
	}
}
