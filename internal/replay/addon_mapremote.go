package replay

import (
	"fmt"
	"log"
	"path"
	"strings"
)

// Path map rule:
//   1. mapFrom.Path /hello and mapTo.Path /world
//     /hello => /world
//   2. mapFrom.Path /hello/* and mapTo.Path /world
//     /hello => /world
//     /hello/abc => /world/abc

type mapRemoteTo struct {
	Protocol string `json:"protocol"`
	Host     string `json:"host"`
	Path     string `json:"path"`
}

type mapRemoteItem struct {
	From   *mapFrom     `json:"from"`
	To     *mapRemoteTo `json:"to"`
	Enable bool         `json:"enable"`
}

func (item *mapRemoteItem) match(req *RequestContext) bool {
	if !item.Enable {
		return false
	}
	return item.From.match(req.Request)
}

func (item *mapRemoteItem) replace(req *RequestContext) error {
	request := req.Request
	if item.To.Protocol != "" {
		request.URL.Scheme = item.To.Protocol
	}
	if item.To.Host != "" {
		request.URL.Host = item.To.Host
		request.Host = item.To.Host
	}
	if item.To.Path != "" {
		if item.From.Path != "" && strings.HasSuffix(item.From.Path, "/*") {
			subPath := request.URL.Path[len(item.From.Path)-2:]
			request.URL.Path = path.Join("/", item.To.Path, subPath)
		} else {
			request.URL.Path = path.Join("/", item.To.Path)
		}
	}

	key, err := buildKey(request, req.Body)
	if err != nil {
		return err
	}
	req.Key = key
	return nil
}

type MapRemote struct {
	BasePlugin
	Items  []*mapRemoteItem `json:"items"`
	Enable bool             `json:"enable"`
}

func (mr *MapRemote) OnRequest(ctx *RequestContext) error {
	if !mr.Enable {
		return nil
	}
	for _, item := range mr.Items {
		if item.match(ctx) {
			before := ctx.Request.URL.String()
			if err := item.replace(ctx); err != nil {
				return err
			}
			after := ctx.Request.URL.String()
			log.Printf("map remote %s to %s", before, after)
			return nil
		}
	}
	return nil
}

func (mr *MapRemote) validate() error {
	for i, item := range mr.Items {
		if item.From == nil {
			return fmt.Errorf("%d no item.From", i)
		}
		if item.From.Protocol != "" && item.From.Protocol != "http" && item.From.Protocol != "https" {
			return fmt.Errorf("%d invalid item.From.Protocol %s", i, item.From.Protocol)
		}
		if item.To == nil {
			return fmt.Errorf("%d no item.To", i)
		}
		if item.To.Protocol == "" && item.To.Host == "" && item.To.Path == "" {
			return fmt.Errorf("%d empty item.To", i)
		}
		if item.To.Protocol != "" && item.To.Protocol != "http" && item.To.Protocol != "https" {
			return fmt.Errorf("%d invalid item.To.Protocol %s", i, item.To.Protocol)
		}
	}
	return nil
}

func NewMapRemoteFromFile(filename string) (*MapRemote, error) {
	var mapRemote MapRemote
	if err := newStructFromFile(filename, &mapRemote); err != nil {
		return nil, err
	}
	if mapRemote.PluginName == "" {
		mapRemote.PluginName = "map-remote"
	}
	if err := mapRemote.validate(); err != nil {
		return nil, err
	}
	return &mapRemote, nil
}
