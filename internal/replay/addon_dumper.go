package replay

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"unicode"
)

// Dumper logs request and response details.
// level 0: headers; level 1: headers + body.
type Dumper struct {
	BasePlugin
	out   io.Writer
	level int
}

func NewDumper(out io.Writer, level int) *Dumper {
	if level != 0 && level != 1 {
		level = 0
	}
	return &Dumper{BasePlugin: BasePlugin{PluginName: "dumper"}, out: out, level: level}
}

func NewDumperWithFilename(filename string, level int) (*Dumper, error) {
	out, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	return NewDumper(out, level), nil
}

func (d *Dumper) OnResponse(ctx *RequestContext, stored *StoredResponse) error {
	if ctx == nil || ctx.Request == nil {
		return nil
	}
	d.dump(ctx, stored)
	return nil
}

func (d *Dumper) dump(ctx *RequestContext, stored *StoredResponse) {
	buf := bytes.NewBuffer(make([]byte, 0))
	req := ctx.Request

	fmt.Fprintf(buf, "%s %s %s\r\n", req.Method, req.URL.RequestURI(), req.Proto)
	host := requestHost(req)
	if host != "" {
		fmt.Fprintf(buf, "Host: %s\r\n", host)
	}
	if len(req.TransferEncoding) > 0 {
		fmt.Fprintf(buf, "Transfer-Encoding: %s\r\n", strings.Join(req.TransferEncoding, ","))
	}
	if req.Close {
		fmt.Fprintf(buf, "Connection: close\r\n")
	}
	_ = req.Header.WriteSubset(buf, nil)
	buf.WriteString("\r\n")

	if d.level == 1 && len(ctx.Body) > 0 && canPrint(ctx.Body) {
		buf.Write(ctx.Body)
		buf.WriteString("\r\n\r\n")
	}

	if stored != nil {
		fmt.Fprintf(buf, "%v %v %v\r\n", req.Proto, stored.StatusCode, http.StatusText(stored.StatusCode))
		storedHeaderToHTTP(stored.Headers).Write(buf)
		buf.WriteString("\r\n")

		if d.level == 1 && stored.BodyBase64 != "" && isTextHeaders(stored.Headers) {
			body, err := base64.StdEncoding.DecodeString(stored.BodyBase64)
			if err == nil && len(body) > 0 {
				buf.Write(body)
				buf.WriteString("\r\n\r\n")
			}
		}
	}

	buf.WriteString("\r\n\r\n")
	_, _ = d.out.Write(buf.Bytes())
}

func canPrint(content []byte) bool {
	for _, c := range string(content) {
		if !unicode.IsPrint(c) && !unicode.IsSpace(c) {
			return false
		}
	}
	return true
}

func storedHeaderToHTTP(headers []Header) http.Header {
	out := make(http.Header, len(headers))
	for _, header := range headers {
		out.Add(header.Key, header.Value)
	}
	return out
}

func isTextHeaders(headers []Header) bool {
	for _, header := range headers {
		if !strings.EqualFold(header.Key, "Content-Type") {
			continue
		}
		contentType := strings.ToLower(header.Value)
		if strings.Contains(contentType, "text/") ||
			strings.Contains(contentType, "application/json") ||
			strings.Contains(contentType, "application/xml") ||
			strings.Contains(contentType, "application/javascript") {
			return true
		}
	}
	return false
}
