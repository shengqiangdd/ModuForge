package middleware

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
)

var gzipPool = sync.Pool{
	New: func() interface{} {
		w, _ := gzip.NewWriterLevel(nil, gzip.DefaultCompression)
		return w
	},
}

// Compress Gzip压缩中间件
func Compress(c fiber.Ctx) error {
	// 检查客户端是否支持gzip
	acceptEncoding := c.Get("Accept-Encoding")
	if !strings.Contains(acceptEncoding, "gzip") {
		return c.Next()
	}

	// 获取响应体
	if err := c.Next(); err != nil {
		return err
	}

	body := c.Response().Body()
	if len(body) == 0 {
		return nil
	}

	// 如果已经压缩，跳过
	if c.Get("Content-Encoding") == "gzip" {
		return nil
	}

	// 压缩
	var buf bytes.Buffer
	w := gzipPool.Get().(*gzip.Writer)
	w.Reset(&buf)

	if _, err := w.Write(body); err != nil {
		gzipPool.Put(w)
		return err
	}

	if err := w.Close(); err != nil {
		gzipPool.Put(w)
		return err
	}
	gzipPool.Put(w)

	// 设置响应头
	c.Set("Content-Encoding", "gzip")
	c.Set("Vary", "Accept-Encoding")
	c.Set("Content-Length", fmt.Sprintf("%d", buf.Len()))

	// 发送压缩后的响应
	return c.Status(c.Response().StatusCode()).Send(buf.Bytes())
}
