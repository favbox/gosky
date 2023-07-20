package app

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/favbox/gobox/wind/internal/nocopy"
	"github.com/favbox/gobox/wind/pkg/common/utils"
	"github.com/favbox/gobox/wind/pkg/network"
	"github.com/favbox/gobox/wind/pkg/protocol/consts"
)

// PathRewriteFunc 必须返回基于任意上下文信息的新请求路径。
//
// 用于将当前请求路径转换为相对于 FS.Root 的本地文件系统路径。
//
// 基于安全考虑，返回路径不能包含 '/../' 这种在 Fs.Root 之外的子字符串。
type PathRewriteFunc func(ctx *RequestContext) []byte

// FS 表示为本地文件系统中静态文件提供服务的请求处理器的设置。
//
// 不要值拷贝 FS，而要创建实例。
type FS struct {
	noCopy nocopy.NoCopy

	// 静态文件服务的根目录。
	Root string

	// 访问目录时尝试打开的索引文件名称切片。
	//
	// 例如：
	//
	//	* index.html
	//	* index.htm
	//	* my-super-index.html
	//
	// 默认索引名称列表为空。
	IndexNames []string

	// 目录无 IndexNames 匹配文件时，要自动生成索引页?
	//
	// 多文件目录（超过 1K）生成索引页会很慢，故默认不开启。
	GenerateIndexPages bool

	// 是否压缩响应？
	//
	// 若启用压缩，能够最小化服务器的 CPU 用量。
	// 开启后将会添加 CompressedFileSuffix 后缀到原始文件名并另存为新文件。
	// 因此，开启前要授予根目录及所有子目录写权限。
	Compress bool

	// 要添加到缓存压缩文件名称的后缀。
	//
	// 仅在 Compress 开启时生效，默认值为 FSCompressedFileSuffix。
	CompressedFileSuffix string

	// 不活跃文件处理器的过期时长。
	//
	// 默认值为 FSHandlerCacheDuration。
	CacheDuration time.Duration

	// 启用字节范围请求？
	//
	// 默认为禁用。
	AcceptByteRange bool

	// 路径重写函数。
	//
	// 默认不重写。
	PathRewrite PathRewriteFunc

	// 当文件不存在时可自定义处理方式。
	//
	// 默认返回 “无法打开请求路径”
	PathNotFound HandlerFunc

	once sync.Once
	h    HandlerFunc
}

// NewRequestHandler 返回当前 FS 的请求处理器。
//
// 返回的处理器会进行缓存，缓存时长为 FS.CacheDuration。
// 若 FS.Root 文件夹有大量文件，请通过 'ulimit -n' 提升文件打开数。
//
// 不要从单个 FS 实例创建多个请求处理器 - 只需复用一个请求处理器即可。
func (fs *FS) NewRequestHandler() HandlerFunc {
	fs.once.Do(fs.initRequestHandler)
	return fs.h
}

func (fs *FS) initRequestHandler() {
	root := fs.Root

	// 若根目录为空，则提供当前工作目录的文件服务
	if len(root) == 0 {
		root = "."
	}

	// 删除根路径的尾随斜线
	for len(root) > 0 && root[len(root)-1] == '/' {
		root = root[:len(root)-1]
	}

	cacheDuration := fs.CacheDuration
	if cacheDuration <= 0 {
		cacheDuration = consts.FSHandlerCacheDuration
	}
	compressedFileSuffix := fs.CompressedFileSuffix
	if len(compressedFileSuffix) == 0 {
		compressedFileSuffix = consts.FSCompressedFileSuffix
	}

	h := &fsHandler{}

	fs.h = h.handleRequest
}

type fsHandler struct {
	root                 string
	indexNames           []string
	pathRewrite          PathRewriteFunc
	pathNotFound         HandlerFunc
	generateIndexPages   bool
	compress             bool
	acceptByteRange      bool
	cacheDuration        time.Duration
	compressedFileSuffix string

	cache           map[string]*fsFile
	compressedCache map[string]*fsFile
	cacheLock       sync.Mutex

	smallFileReaderPool sync.Pool
}

type fsFile struct {
	h             *fsHandler
	f             *os.File
	dirIndex      []byte
	contentType   string
	contentLength int
	compressed    bool

	lastModified    time.Time
	lastModifiedStr []byte

	t            time.Time
	readersCount int

	bigFiles     []*bigFileReader
	bigFilesLock sync.Mutex
}

func (ff *fsFile) NewReader() (io.Reader, error) {
	if ff.isBig() {
		r, err := ff.bigFileReader()
		if err != nil {
			ff.decReadersCount()
		}
		return r, err
	}
	return ff.smallFileReader(), nil
}

func (ff *fsFile) Release() {
	if ff.f != nil {
		ff.f.Close()

		if ff.isBig() {
			ff.bigFilesLock.Lock()
			for _, r := range ff.bigFiles {
				r.f.Close()
			}
			ff.bigFilesLock.Unlock()
		}
	}
}

func (ff *fsFile) isBig() bool {
	return ff.contentLength > consts.MaxSmallFileSize && len(ff.dirIndex) == 0
}

func (ff *fsFile) bigFileReader() (io.Reader, error) {
	if ff.f == nil {
		panic("BUG: ff.f 不能为空")
	}

	var r io.Reader

	ff.bigFilesLock.Lock()
	n := len(ff.bigFiles)
	if n > 0 {
		r = ff.bigFiles[n-1]
		ff.bigFiles = ff.bigFiles[:n-1]
	}
	ff.bigFilesLock.Unlock()

	if r != nil {
		return r, nil
	}

	f, err := os.Open(ff.f.Name())
	if err != nil {
		return nil, fmt.Errorf("无法打开已打开的文件：%s", err)
	}

	return &bigFileReader{
		f:  f,
		ff: ff,
		r:  f,
	}, nil
}

func (ff *fsFile) smallFileReader() io.Reader {
	v := ff.h.smallFileReaderPool.Get()
	if v == nil {
		v = &fsSmallFileReader{}
	}
	r := v.(*fsSmallFileReader)

}

func (ff *fsFile) decReadersCount() {
	ff.h.cacheLock.Lock()
	defer ff.h.cacheLock.Unlock()
	ff.readersCount--
	if ff.readersCount < 0 {
		panic("BUG: fsFile.readersCount 为负数！")
	}
}

type byteRangeUpdater interface {
	UpdateByteRange(startPos, endPos int) error
}

// 尝试按需发送大文件。
type bigFileReader struct {
	f  *os.File
	ff *fsFile
	r  io.Reader
	lr io.LimitedReader
}

func (r *bigFileReader) UpdateByteRange(startPos, endPos int) error {
	if _, err := r.f.Seek(int64(startPos), 0); err != nil {
		return err
	}
	r.r = &r.lr
	r.lr.R = r.f
	r.lr.N = int64(endPos - startPos + 1)
	return nil
}

func (r *bigFileReader) Read(p []byte) (int, error) {
	return r.r.Read(p)
}

func (r *bigFileReader) WriteTo(w io.Writer) (n int64, err error) {
	if rf, ok := w.(io.ReaderFrom); ok {
		// 快路径。Sendfile 一定被触发。
		return rf.ReadFrom(r.r)
	}
	zw := network.NewWriter(w)
	// 慢路径
	return utils.CopyZeroAlloc(zw, r.r)
}

func (r *bigFileReader) Close() error {
	r.r = r.f
	n, err := r.f.Seek(0, 0)
	if err == nil {
		if n != 0 {
			panic("BUG: File.Seek(0, 0) 返回 (non-zero, nil)")
		}

		ff := r.ff
		ff.bigFilesLock.Lock()
		ff.bigFiles = append(ff.bigFiles, r)
		ff.bigFilesLock.Unlock()
	} else {
		r.f.Close()
	}
	r.ff.decReadersCount()
	return err
}

type fsSmallFileReader struct {
	ff       *fsFile
	startPos int
	endPos   int
}

func (r *fsSmallFileReader) UpdateByteRange(startPos, endPos int) error {
	r.startPos = startPos
	r.endPos = endPos
	return nil
}

func (r *fsSmallFileReader) Read(p []byte) (int, error) {
	tailLen := r.endPos - r.startPos
	if tailLen <= 0 {
		return 0, io.EOF
	}
	if len(p) > tailLen {
		p = p[:tailLen]
	}

	ff := r.ff
	if ff.f != nil {
		n, err := ff.f.ReadAt(p, int64(r.startPos))
		r.startPos += n
		return n, err
	}

	n := copy(p, ff.dirIndex[r.startPos:])
	r.startPos += n
	return n, nil
}

func (r *fsSmallFileReader) WriteTo(w io.Writer) (int64, error) {
	ff := r.ff

	var n int
	var err error
	if ff.f == nil {
		n, err = w.Write(ff.dirIndex[r.startPos:r.endPos])
		return int64(n), err
	}
}

func (r *fsSmallFileReader) Close() error {
	//TODO implement me
	panic("implement me")
}

var (
	rootFSOnce sync.Once
	rootFS     = &FS{
		Root:               "/",
		GenerateIndexPages: true,
		Compress:           true,
	}
	rootFSHandler HandlerFunc
)

func ServeFile(ctx *RequestContext, path string) {
	rootFSOnce.Do(func() {
		rootFSHandler = rootFS.NewRequestHandler()
	})
	// TODO 未完待续
}
