package mock

import (
	"bytes"
	"testing"

	"github.com/favbox/gosky/wind/pkg/common/test/assert"
)

func TestExtWriter(t *testing.T) {
	b1 := []byte("abcdef4343")
	buf := new(bytes.Buffer)
	isFinal := false
	w := &ExtWriter{
		Buf:     buf,
		IsFinal: &isFinal,
	}

	// write
	n, err := w.Write(b1)
	assert.DeepEqual(t, nil, err)
	assert.DeepEqual(t, len(b1), n)

	// flush
	err = w.Flush()
	assert.DeepEqual(t, nil, err)
	assert.DeepEqual(t, b1, w.Buf.Bytes())

	// setbody
	b2 := []byte("abc")
	w.SetBody(b2)
	err = w.Flush()
	assert.DeepEqual(t, nil, err)
	assert.DeepEqual(t, b2, w.Buf.Bytes())

	_ = w.Finalize()
	assert.DeepEqual(t, true, *(w.IsFinal))
}
