package replay

import (
	"encoding/base64"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

type mapLocalTo struct {
	Path string `json:"path"`
}

type mapLocalItem struct {
	From   *mapFrom    `json:"from"`
	To     *mapLocalTo `json:"to"`
	Enable bool        `json:"enable"`
}

func (item *mapLocalItem) match(req *RequestContext) bool {
	if !item.Enable {
		return false
	}
	return item.From.match(req.Request)
}

func (item *mapLocalItem) response(req *RequestContext) (string, *StoredResponse) {
	getStat := func(filepath string) (fs.FileInfo, *StoredResponse) {
		stat, err := os.Stat(filepath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, &StoredResponse{StatusCode: http.StatusNotFound}
			}
			log.Printf("map local %s stat error: %v", filepath, err)
			return nil, &StoredResponse{StatusCode: http.StatusInternalServerError}
		}
		return stat, nil
	}

	respFile := func(filepath string) *StoredResponse {
		body, err := os.ReadFile(filepath)
		if err != nil {
			log.Printf("map local %s read error: %v", filepath, err)
			return &StoredResponse{StatusCode: http.StatusInternalServerError}
		}
		encoded := ""
		if len(body) > 0 {
			encoded = base64.StdEncoding.EncodeToString(body)
		}
		return &StoredResponse{
			StatusCode: http.StatusOK,
			BodyBase64: encoded,
		}
	}

	stat, resp := getStat(item.To.Path)
	if resp != nil {
		return item.To.Path, resp
	}

	if !stat.IsDir() {
		return item.To.Path, respFile(item.To.Path)
	}

	subPath := req.Request.URL.Path
	if item.From.Path != "" && strings.HasSuffix(item.From.Path, "/*") {
		subPath = req.Request.URL.Path[len(item.From.Path)-2:]
	}
	subPath = strings.TrimPrefix(subPath, "/")
	filepath := path.Join(item.To.Path, subPath)

	stat, resp = getStat(filepath)
	if resp != nil {
		return filepath, resp
	}

	if !stat.IsDir() {
		return filepath, respFile(filepath)
	}
	log.Printf("map local %s should be file", filepath)
	return filepath, &StoredResponse{StatusCode: http.StatusInternalServerError}
}

type MapLocal struct {
	BasePlugin
	Items  []*mapLocalItem `json:"items"`
	Enable bool            `json:"enable"`
}

func (ml *MapLocal) OnRequest(ctx *RequestContext) error {
	if !ml.Enable {
		return nil
	}
	for _, item := range ml.Items {
		if item.match(ctx) {
			aurl := ctx.Request.URL.String()
			localfile, resp := item.response(ctx)
			log.Printf("map local %s to %s", aurl, localfile)
			ctx.Response = resp
			return nil
		}
	}
	return nil
}

func (ml *MapLocal) validate() error {
	for i, item := range ml.Items {
		if item.From == nil {
			return fmt.Errorf("%d no item.From", i)
		}
		if item.From.Protocol != "" && item.From.Protocol != "http" && item.From.Protocol != "https" {
			return fmt.Errorf("%d invalid item.From.Protocol %s", i, item.From.Protocol)
		}
		if item.To == nil {
			return fmt.Errorf("%d no item.To", i)
		}
		if item.To.Path == "" {
			return fmt.Errorf("%d empty item.To.Path", i)
		}
	}
	return nil
}

func NewMapLocalFromFile(filename string) (*MapLocal, error) {
	var mapLocal MapLocal
	if err := newStructFromFile(filename, &mapLocal); err != nil {
		return nil, err
	}
	if mapLocal.PluginName == "" {
		mapLocal.PluginName = "map-local"
	}
	if err := mapLocal.validate(); err != nil {
		return nil, err
	}
	return &mapLocal, nil
}
