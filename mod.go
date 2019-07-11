package kuu

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/jinzhu/inflection"
	"path"
	"strings"
)

var tableNames = make(map[string]string)

func init() {
	gorm.DefaultTableNameHandler = func(db *gorm.DB, defaultTableName string) string {
		v, ok := tableNames[defaultTableName]
		if !ok || v == "" {
			WARN("自定义表名：%s", defaultTableName)
			return defaultTableName
		}
		return v
	}
}

// Mod
type Mod struct {
	Code        string
	Middlewares gin.HandlersChain
	Routes      RoutesInfo
	Models      []interface{}
	AfterImport func()
}

// Import
func (e *Engine) Import(mods ...*Mod) {
	migrate := C().GetBool("gorm:migrate")
	for _, mod := range mods {
		for _, middleware := range mod.Middlewares {
			if middleware != nil {
				e.Engine.Use(middleware)
			}
		}
	}
	for _, mod := range mods {
		if mod.Code == "" {
			PANIC("模块编码不能为空")
		}
		mod.Code = strings.ToLower(mod.Code)
		for _, route := range mod.Routes {
			if route.Path == "" || route.HandlerFunc == nil {
				PANIC("Route path and handler can't be nil")
			}
			if route.Method == "" {
				route.Method = "GET"
			}
			var routePath string
			if route.IgnorePrefix {
				routePath = path.Join(route.Path)
			} else {
				routePath = path.Join(C().GetString("prefix"), route.Path)
			}
			if route.Method == "*" {
				e.Any(routePath, route.HandlerFunc)
			} else {
				e.Handle(route.Method, routePath, route.HandlerFunc)
			}
		}
		for _, model := range mod.Models {
			desc := RESTful(e, model)
			if meta := parseMetadata(model); meta != nil {
				meta.RestDesc = desc
				defaultTableName := gorm.ToTableName(meta.Name)
				pluralTableName := inflection.Plural(defaultTableName)

				tableName := fmt.Sprintf("%s_%s", mod.Code, meta.Name)
				tableNames[defaultTableName] = tableName
				tableNames[pluralTableName] = tableName
			}
			if migrate {
				DB().AutoMigrate(model)
			}
		}
		if mod.AfterImport != nil {
			mod.AfterImport()
		}
	}
}
