package controller

import (
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
)

func parseInt64Param(hc *app.RequestContext, param string) int64 {
	val := hc.Param(param)
	if val == "" {
		return 0
	}
	id, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

func parseInt64Query(hc *app.RequestContext, key string) int64 {
	v := hc.Query(key)
	if v == "" {
		return 0
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

func parseIntQueryDefault(hc *app.RequestContext, key string, def int) int {
	v := hc.Query(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
