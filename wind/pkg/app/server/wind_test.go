package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/favbox/gosky/wind/pkg/app"
	errs "github.com/favbox/gosky/wind/pkg/common/errors"
	"github.com/favbox/gosky/wind/pkg/common/test/assert"
	"github.com/favbox/gosky/wind/pkg/common/test/mock"
	"github.com/favbox/gosky/wind/pkg/common/utils"
	"github.com/favbox/gosky/wind/pkg/network"
	"github.com/favbox/gosky/wind/pkg/protocol"
	"github.com/favbox/gosky/wind/pkg/protocol/consts"
	"github.com/favbox/gosky/wind/pkg/protocol/http1/req"
	"github.com/favbox/gosky/wind/pkg/protocol/http1/resp"
)

func TestWind_Spin_Close(t *testing.T) {
	wind := New(WithHostPorts("localhost:8888"))
	wind.GET("/test", func(c context.Context, ctx *app.RequestContext) {
		path := ctx.Request.URI().PathOriginal()
		ctx.SetBodyString(string(path))
	})

	testInt := uint32(0)
	wind.Engine.OnShutdown = append(wind.Engine.OnShutdown, func(ctx context.Context) {
		atomic.StoreUint32(&testInt, 1)
	})

	go wind.Spin()
	time.Sleep(100 * time.Millisecond)

	wind.Close()
	resp, err := http.Get("http://localhost:8888/test")
	assert.NotNil(t, err)
	assert.Nil(t, resp)
	assert.DeepEqual(t, uint32(0), atomic.LoadUint32(&testInt))
}

func TestWind_Spin_Shutdown(t *testing.T) {
	wind := New(WithHostPorts("127.0.0.1:8888"))
	wind.GET("/test", func(c context.Context, ctx *app.RequestContext) {
		time.Sleep(time.Second * 2)
		path := ctx.Request.URI().PathOriginal()
		ctx.SetBodyString(string(path))
	})
	wind.GET("/test2", func(c context.Context, ctx *app.RequestContext) {})

	shutdownHook1 := uint32(0)
	shutdownHook2 := uint32(0)
	shutdownHook3 := uint32(0)
	wind.Engine.OnShutdown = append(wind.Engine.OnShutdown, func(ctx context.Context) {
		atomic.StoreUint32(&shutdownHook1, 1)
	})
	wind.Engine.OnShutdown = append(wind.Engine.OnShutdown, func(ctx context.Context) {
		atomic.StoreUint32(&shutdownHook2, 2)
	})
	wind.Engine.OnShutdown = append(wind.Engine.OnShutdown, func(ctx context.Context) {
		time.Sleep(2 * time.Second)
		atomic.StoreUint32(&shutdownHook3, 3)
	})

	go wind.Spin()
	time.Sleep(time.Millisecond)

	hc := http.Client{Timeout: time.Second}
	var err error
	var resp *http.Response
	ch := make(chan struct{})
	ch2 := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Millisecond * 100)
		defer ticker.Stop()
		for range ticker.C {
			t.Logf("[%v]开始监听\n", time.Now())
			_, err2 := hc.Get("http://127.0.0.1:8888/test2")
			if err2 != nil {
				t.Logf("[%v]监听已关闭: %v", time.Now(), err2)
				ch2 <- struct{}{}
				break
			}
		}
	}()
	go func() {
		t.Logf("[%v]开始请求\n", time.Now())
		resp, err = http.Get("http://localhost:8888/test")
		t.Logf("[%v]请求结束\n", time.Now())
		ch <- struct{}{}
	}()

	time.Sleep(time.Second * 1)
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	t.Logf("[%v]开始退出\n", start)
	wind.Shutdown(ctx)
	end := time.Now()
	t.Logf("[%v]退出完成", end)

	<-ch
	assert.Nil(t, err)
	assert.NotNil(t, resp)
	assert.DeepEqual(t, true, resp.Close)
	assert.DeepEqual(t, uint32(1), atomic.LoadUint32(&shutdownHook1))
	assert.DeepEqual(t, uint32(2), atomic.LoadUint32(&shutdownHook2))
	assert.DeepEqual(t, uint32(3), atomic.LoadUint32(&shutdownHook3))

	<-ch2

	cancel()
}

func TestLoadHTMLGlob(t *testing.T) {
	wind := New(WithMaxRequestBodySize(15), WithHostPorts("127.0.0.1:8888"))
	wind.Delims("{[{", "}]}")
	wind.LoadHTMLGlob("../../common/testdata/template/index.tmpl")
	wind.GET("/index", func(c context.Context, ctx *app.RequestContext) {
		ctx.HTML(consts.StatusOK, "index.tmpl", utils.H{
			"title": "主站",
		})
	})
	go wind.Run()
	time.Sleep(200 * time.Millisecond)
	resp, _ := http.Get("http://127.0.0.1:8888/index")
	assert.DeepEqual(t, consts.StatusOK, resp.StatusCode)
	b := make([]byte, 100)
	n, _ := resp.Body.Read(b)
	assert.DeepEqual(t, `<html><h1>主站</h1></html>`, string(b[:n]))
}

func TestLoadHTMLFiles(t *testing.T) {
	wind := New(WithMaxRequestBodySize(15), WithHostPorts("127.0.0.1:8888"))
	wind.Delims("{[{", "}]}")
	wind.SetFuncMap(template.FuncMap{
		"formatAsDate": formatAsDate,
	})
	wind.LoadHTMLFiles("../../common/testdata/template/htmltemplate.html", "../../common/testdata/template/index.tmpl")
	wind.GET("/raw", func(c context.Context, ctx *app.RequestContext) {
		ctx.HTML(consts.StatusOK, "htmltemplate.html", utils.H{
			"now": time.Date(2017, 7, 1, 0, 0, 0, 0, time.UTC),
		})
	})

	go wind.Run()
	time.Sleep(200 * time.Millisecond)
	resp, _ := http.Get("http://localhost:8888/raw")
	assert.DeepEqual(t, consts.StatusOK, resp.StatusCode)
	b := make([]byte, 100)
	n, _ := resp.Body.Read(b)
	assert.DeepEqual(t, "<h1>Date: 2017/07/01</h1>", string(b[:n]))
}

func formatAsDate(t time.Time) string {
	year, month, day := t.Date()
	return fmt.Sprintf("%d/%02d/%02d", year, month, day)
}

func TestWind_Engine_Use(t *testing.T) {
	router := New()
	router.Use(func(c context.Context, ctx *app.RequestContext) {})
	assert.DeepEqual(t, 1, len(router.Handlers))
	router.Use(func(c context.Context, ctx *app.RequestContext) {})
	assert.DeepEqual(t, 2, len(router.Handlers))
}

func TestWind_Engine_GetServerName(t *testing.T) {
	router := New()
	assert.DeepEqual(t, []byte("wind"), router.GetServerName())
	router = New()
	router.Name = "test_name"
	assert.DeepEqual(t, []byte(string(router.GetServerName())), router.GetServerName())
}

func TestServer_Run(t *testing.T) {
	wind := New(WithHostPorts("localhost:8888"))
	wind.GET("/test", func(c context.Context, ctx *app.RequestContext) {
		path := ctx.Request.URI().PathOriginal()
		ctx.SetBodyString(string(path))
	})
	wind.POST("/redirect", func(c context.Context, ctx *app.RequestContext) {
		ctx.Redirect(consts.StatusMovedPermanently, []byte("http://localhost:8888/test"))
	})
	go wind.Run()
	time.Sleep(100 * time.Millisecond)
	resp, err := http.Get("http://localhost:8888/test")
	assert.Nil(t, err)
	assert.DeepEqual(t, consts.StatusOK, resp.StatusCode)
	b := make([]byte, 5)
	resp.Body.Read(b)
	assert.DeepEqual(t, "/test", string(b))

	resp, err = http.Get("http://localhost:8888/foo")
	assert.Nil(t, err)
	assert.DeepEqual(t, consts.StatusNotFound, resp.StatusCode)

	resp, err = http.Post("http://127.0.0.1:8888/redirect", "", nil)
	assert.Nil(t, err)
	assert.DeepEqual(t, consts.StatusOK, resp.StatusCode)
	b = make([]byte, 5)
	resp.Body.Read(b)
	assert.DeepEqual(t, "/test", string(b))

	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	_ = wind.Shutdown(ctx)
}

func TestNotAbsolutePath(t *testing.T) {
	wind := New(WithHostPorts("127.0.0.1:8888"))
	wind.POST("/", func(c context.Context, ctx *app.RequestContext) {
		ctx.Write(ctx.Request.Body())
	})
	wind.POST("/a", func(c context.Context, ctx *app.RequestContext) {
		ctx.Write(ctx.Request.Body())
	})
	go wind.Run()
	time.Sleep(200 * time.Microsecond)

	s := "POST ?a=b HTTP/1.1\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	zr := mock.NewZeroCopyReader(s)

	ctx := app.NewContext(0)
	if err := req.Read(&ctx.Request, zr); err != nil {
		t.Fatalf("不期待的错误：%s", err)
	}
	wind.ServeHTTP(context.Background(), ctx)
	assert.DeepEqual(t, consts.StatusOK, ctx.Response.StatusCode())
	assert.DeepEqual(t, ctx.Request.Body(), ctx.Response.Body())

	s = "POST a?a=b HTTP/1.1\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	zr = mock.NewZeroCopyReader(s)

	ctx = app.NewContext(0)
	if err := req.Read(&ctx.Request, zr); err != nil {
		t.Fatalf("不期待的错误：%s", err)
	}
	wind.ServeHTTP(context.Background(), ctx)
	assert.DeepEqual(t, consts.StatusOK, ctx.Response.StatusCode())
	assert.DeepEqual(t, ctx.Request.Body(), ctx.Response.Body())
}

// 靠自 router
var default400Body = []byte("400 错误请求")

func TestNotAbsolutePathWithRawPath(t *testing.T) {
	engine := New(WithHostPorts("127.0.0.1:9991"), WithUseRawPath(true))
	engine.POST("/", func(c context.Context, ctx *app.RequestContext) {
	})
	engine.POST("/a", func(c context.Context, ctx *app.RequestContext) {
	})
	go engine.Run()
	time.Sleep(200 * time.Microsecond)

	s := "POST ?a=b HTTP/1.1\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	zr := mock.NewZeroCopyReader(s)

	ctx := app.NewContext(0)
	if err := req.Read(&ctx.Request, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	engine.ServeHTTP(context.Background(), ctx)
	assert.DeepEqual(t, consts.StatusBadRequest, ctx.Response.StatusCode())
	assert.DeepEqual(t, default400Body, ctx.Response.Body())

	s = "POST a?a=b HTTP/1.1\r\nContent-Length: 5\r\nContent-Type: foo/bar\r\n\r\nabcdef4343"
	zr = mock.NewZeroCopyReader(s)

	ctx = app.NewContext(0)
	if err := req.Read(&ctx.Request, zr); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	engine.ServeHTTP(context.Background(), ctx)
	assert.DeepEqual(t, consts.StatusBadRequest, ctx.Response.StatusCode())
	assert.DeepEqual(t, default400Body, ctx.Response.Body())
}

func TestWithBasePath(t *testing.T) {
	engine := New(WithBasePath("/wind"), WithHostPorts("127.0.0.1:19898"))
	engine.POST("/test", func(c context.Context, ctx *app.RequestContext) {})
	go engine.Run()
	time.Sleep(500 * time.Microsecond)
	var r http.Request
	r.ParseForm()
	r.Form.Add("xxxxxx", "xxx")
	body := strings.NewReader(r.Form.Encode())
	resp, err := http.Post("http://127.0.0.1:19898/wind/test", "application/x-www-form-urlencoded", body)
	assert.Nil(t, err)
	assert.DeepEqual(t, consts.StatusOK, resp.StatusCode)
}

func TestNotEnoughBodySize(t *testing.T) {
	engine := New(WithMaxRequestBodySize(5), WithHostPorts("127.0.0.1:8889"))
	engine.POST("/test", func(c context.Context, ctx *app.RequestContext) {})
	go engine.Run()
	time.Sleep(200 * time.Microsecond)
	var r http.Request
	r.ParseForm()
	r.Form.Add("xxxxxx", "xxx") // 正文大小 10 "xxxxxx=xxx"
	body := strings.NewReader(r.Form.Encode())
	resp, err := http.Post("http://127.0.0.1:8889/test", "application/x-www-form-urlencoded", body)
	assert.Nil(t, err)
	assert.DeepEqual(t, 413, resp.StatusCode)
	bodyBytes, _ := io.ReadAll(resp.Body)
	assert.DeepEqual(t, "请求实体过大", string(bodyBytes))
}

func TestEnoughBodySize(t *testing.T) {
	engine := New(WithMaxRequestBodySize(15), WithHostPorts("127.0.0.1:8892"))
	engine.POST("/test", func(c context.Context, ctx *app.RequestContext) {
	})
	go engine.Run()
	time.Sleep(200 * time.Microsecond)
	var r http.Request
	r.ParseForm()
	r.Form.Add("xxxxxx", "xxx")
	body := strings.NewReader(r.Form.Encode())
	resp, _ := http.Post("http://127.0.0.1:8892/test", "application/x-www-form-urlencoded", body)
	assert.DeepEqual(t, consts.StatusOK, resp.StatusCode)
}

func TestRequestCtxHijack(t *testing.T) {
	hijackStartCh := make(chan struct{})
	hijackStopCh := make(chan struct{})
	wind := New()
	wind.Init()

	wind.GET("/foo", func(c context.Context, ctx *app.RequestContext) {
		if ctx.Hijacked() {
			t.Error("connection mustn't be hijacked")
		}
		ctx.Hijack(func(c network.Conn) {
			<-hijackStartCh

			b := make([]byte, 1)
			// ping-pong echo via hijacked conn
			for {
				n, err := c.Read(b)
				if n != 1 {
					if err == io.EOF {
						close(hijackStopCh)
						return
					}
					if err != nil {
						t.Errorf("unexpected error: %s", err)
					}
					t.Errorf("unexpected number of bytes read: %d. Expecting 1", n)
				}
				if _, err = c.Write(b); err != nil {
					t.Errorf("unexpected error when writing data: %s", err)
				}
			}
		})
		if !ctx.Hijacked() {
			t.Error("connection must be hijacked")
		}
		ctx.Data(consts.StatusOK, "foo/bar", []byte("hijack it!"))
	})

	hijackedString := "foobar baz hijacked!!!"

	c := mock.NewConn("GET /foo HTTP/1.1\r\nHost: google.com\r\n\r\n" + hijackedString)

	ch := make(chan error)
	go func() {
		ch <- wind.Serve(context.Background(), c)
	}()

	time.Sleep(100 * time.Millisecond)

	close(hijackStartCh)

	if err := <-ch; err != nil {
		if !errors.Is(err, errs.ErrHijacked) {
			t.Fatalf("Unexpected error from serveConn: %s", err)
		}
	}
	verifyResponse(t, c.WriterRecorder(), consts.StatusOK, "foo/bar", "hijack it!")

	select {
	case <-hijackStopCh:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout")
	}

	zw := c.WriterRecorder()
	data, err := zw.ReadBinary(zw.Len())
	if err != nil {
		t.Fatalf("Unexpected error when reading remaining data: %s", err)
	}
	if string(data) != hijackedString {
		t.Fatalf("Unexpected data read after the first response %q. Expecting %q", data, hijackedString)
	}
}

func verifyResponse(t *testing.T, zr network.Reader, expectedStatusCode int, expectedContentType, expectedBody string) {
	var r protocol.Response
	if err := resp.Read(&r, zr); err != nil {
		t.Fatalf("Unexpected error when parsing response: %s", err)
	}

	if !bytes.Equal(r.Body(), []byte(expectedBody)) {
		t.Fatalf("Unexpected body %q. Expected %q", r.Body(), []byte(expectedBody))
	}
	verifyResponseHeader(t, &r.Header, expectedStatusCode, len(r.Body()), expectedContentType, "")
}

func verifyResponseHeader(t *testing.T, h *protocol.ResponseHeader, expectedStatusCode, expectedContentLength int, expectedContentType, expectedContentEncoding string) {
	if h.StatusCode() != expectedStatusCode {
		t.Fatalf("Unexpected status code %d. Expected %d", h.StatusCode(), expectedStatusCode)
	}
	if h.ContentLength() != expectedContentLength {
		t.Fatalf("Unexpected content length %d. Expected %d", h.ContentLength(), expectedContentLength)
	}
	if string(h.ContentType()) != expectedContentType {
		t.Fatalf("Unexpected content type %q. Expected %q", h.ContentType(), expectedContentType)
	}
	if string(h.ContentEncoding()) != expectedContentEncoding {
		t.Fatalf("Unexpected content encoding %q. Expected %q", h.ContentEncoding(), expectedContentEncoding)
	}
}

// TODO 参数不一致
//func TestParamInconsist(t *testing.T) {
//	mapS := sync.Map{}
//	h := New(WithHostPorts("localhost:10091"))
//	h.GET("/:label", func(c context.Context, ctx *app.RequestContext) {
//		label := ctx.Param("label")
//		x, _ := mapS.LoadOrStore(label, label)
//		labelString := x.(string)
//		if label != labelString {
//			t.Errorf("unexpected label: %s, expected return label: %s", label, labelString)
//		}
//	})
//	go h.Run()
//	time.Sleep(time.Millisecond * 50)
//	client, _ := c.NewClient()
//	wg := sync.WaitGroup{}
//	tr := func() {
//		defer wg.Done()
//		for i := 0; i < 5000; i++ {
//			client.Get(context.Background(), nil, "http://localhost:10091/test1")
//		}
//	}
//	ti := func() {
//		defer wg.Done()
//		for i := 0; i < 5000; i++ {
//			client.Get(context.Background(), nil, "http://localhost:10091/test2")
//		}
//	}
//
//	for i := 0; i < 30; i++ {
//		go tr()
//		go ti()
//		wg.Add(2)
//	}
//	wg.Wait()
//}

//func TestDuplicateReleaseBodyStream(t *testing.T) {
//	h := New(WithStreamBody(true), WithHostPorts("localhost:10092"))
//	h.POST("/test", func(ctx context.Context, c *app.RequestContext) {
//		stream := c.RequestBodyStream()
//		c.Response.SetBodyStream(stream, -1)
//	})
//	go h.Spin()
//	time.Sleep(time.Second)
//	client, _ := c.NewClient(c.WithMaxConnsPerHost(1000000), c.WithDialTimeout(time.Minute))
//	bodyBytes := make([]byte, 102388)
//	index := 0
//	letterBytes := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
//	for i := 0; i < 102388; i++ {
//		bodyBytes[i] = letterBytes[index]
//		if i%1969 == 0 && i != 0 {
//			index = index + 1
//		}
//	}
//	body := string(bodyBytes)
//
//	wg := sync.WaitGroup{}
//	testFunc := func() {
//		defer wg.Done()
//		r := protocol.NewRequest("POST", "http://localhost:10092/test", nil)
//		r.SetBodyString(body)
//		resp := protocol.AcquireResponse()
//		err := client.Do(context.Background(), r, resp)
//		if err != nil {
//			t.Errorf("unexpected error: %s", err.Error())
//		}
//		if body != string(resp.Body()) {
//			t.Errorf("unequal body")
//		}
//	}
//
//	for i := 0; i < 10; i++ {
//		wg.Add(1)
//		go testFunc()
//	}
//	wg.Wait()
//}

func TestSrv(t *testing.T) {
	wind := New()
	wind.Any("/", func(c context.Context, ctx *app.RequestContext) {
		ctx.String(consts.StatusOK, "hello %s!", ctx.DefaultQuery("name", "world"))
	})
	wind.StaticFS("/static", &app.FS{
		Root:                 "/Users/zs/Downloads",
		IndexNames:           nil,
		GenerateIndexPages:   true,
		Compress:             true,
		CompressedFileSuffix: "",
		CacheDuration:        0,
		AcceptByteRange:      false,
		PathRewrite:          nil,
		PathNotFound:         nil,
	})

	//err := wind.Run()
	//if err != nil {
	//	t.Fatal(err)
	//}
	go wind.Spin()
	select {}
}
