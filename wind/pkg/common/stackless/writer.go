package stackless

import (
	"fmt"
	"io"

	"github.com/favbox/gosky/wind/pkg/common/bytebufferpool"
	"github.com/favbox/gosky/wind/pkg/common/errors"
)

// Writer 是无堆栈编写器必须遵守的接口。
// 用以节省高并发占用的堆栈空间。
//
// 该接口包含标准库 compress/* 包中 Writers 的子集。
type Writer interface {
	Write(p []byte) (int, error)
	Flush() error
	Close() error
	Reset(w io.Writer)
}

// NewWriterFunc 将 w 转为无堆栈编写器 Writer 的函数。
type NewWriterFunc func(w io.Writer) Writer

// NewWriter 返回用 newWriter 包装 dstW 得到的无堆栈编写器。
//
// 返回的编写器讲数据写入 dstW。
//
// 将大量占用堆栈的写入器封装为无堆栈写入器，可为大量并发运行的 goroutine 节省堆栈空间。
func NewWriter(dstW io.Writer, newWriter NewWriterFunc) Writer {
	w := &writer{
		dstW: dstW,
	}
	w.zw = newWriter(&w.xw)
	return w
}

type op int

const (
	opWrite op = iota
	opFlush
	opClose
	opReset
)

type writer struct {
	dstW io.Writer
	zw   Writer
	xw   xWriter

	err error
	n   int

	p  []byte
	op op
}

func (w *writer) Write(p []byte) (int, error) {
	w.p = p
	err := w.do(opWrite)
	w.p = nil
	return w.n, err
}

func (w *writer) Flush() error {
	return w.do(opFlush)
}

func (w *writer) Close() error {
	return w.do(opClose)
}

func (w *writer) Reset(dstW io.Writer) {
	w.xw.Reset()
	_ = w.do(opReset)
	w.dstW = dstW
}

func (w *writer) do(op op) error {
	w.op = op
	if !stacklessWriterFunc(w) {
		return errHighLoad
	}
	err := w.err
	if err != nil {
		return err
	}
	if w.xw.bb != nil && len(w.xw.bb.B) > 0 {
		_, err = w.dstW.Write(w.xw.bb.B)
	}
	w.xw.Reset()

	return err
}

var bufferPool bytebufferpool.Pool

type xWriter struct {
	bb *bytebufferpool.ByteBuffer
}

func (w *xWriter) Write(p []byte) (int, error) {
	if w.bb == nil {
		w.bb = bufferPool.Get()
	}
	return w.bb.Write(p)
}

func (w *xWriter) Reset() {
	if w.bb != nil {
		bufferPool.Put(w.bb)
		w.bb = nil
	}
}

var errHighLoad = errors.NewPublic("因负载过高，当前无法压缩数据")

var stacklessWriterFunc = NewFunc(writerFunc)

func writerFunc(ctx any) {
	w := ctx.(*writer)
	switch w.op {
	case opWrite:
		w.n, w.err = w.zw.Write(w.p)
	case opFlush:
		w.err = w.zw.Flush()
	case opClose:
		w.err = w.zw.Close()
	case opReset:
		w.zw.Reset(&w.xw)
		w.err = nil
	default:
		panic(fmt.Sprintf("BUG：不期待的操作：%d", w.op))
	}
}
