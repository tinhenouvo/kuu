package kuu

import (
	"github.com/gin-gonic/gin"
	"path"
	"reflect"
	"strings"
	"time"
)

var metadata = make(map[string]*Metadata)

// Mod
type Mod struct {
	Middleware  gin.HandlersChain
	Routes      RoutesInfo
	Models      []interface{}
	AfterImport func()
}

// Import
func (e *Engine) Import(mods ...*Mod) {
	for _, mod := range mods {
		for _, middleware := range mod.Middleware {
			if middleware != nil {
				e.Engine.Use(middleware)
			}
		}
	}
	for _, mod := range mods {
		for _, route := range mod.Routes {
			if route.Path == "" || route.HandlerFunc == nil {
				PANIC("Route path and handler can't be nil")
			}
			if route.Method == "" {
				route.Method = "GET"
			}
			routePath := path.Join(C().GetString("prefix"), route.Path)
			if route.Method == "*" {
				e.Any(routePath, route.HandlerFunc)
			} else {
				e.Handle(route.Method, routePath, route.HandlerFunc)
			}
		}
		for _, model := range mod.Models {
			RESTful(e, model)
			if meta := parseMetadata(model); meta != nil {
				metadata[meta.Name] = meta
			}
		}
		if mod.AfterImport != nil {
			mod.AfterImport()
		}
	}
}

func parseMetadata(value interface{}) (m *Metadata) {
	reflectType := reflect.ValueOf(value).Type()
	for reflectType.Kind() == reflect.Slice || reflectType.Kind() == reflect.Ptr {
		reflectType = reflectType.Elem()
	}

	// Scope value need to be a struct
	if reflectType.Kind() != reflect.Struct {
		return
	}

	m = new(Metadata)
	m.Name = reflectType.Name()
	m.FullName = path.Join(reflectType.PkgPath(), m.Name)
	for i := 0; i < reflectType.NumField(); i++ {
		fieldStruct := reflectType.Field(i)
		displayName := fieldStruct.Tag.Get("displayName")
		if m.DisplayName == "" && displayName != "" {
			m.DisplayName = displayName
		}
		indirectType := fieldStruct.Type
		for indirectType.Kind() == reflect.Ptr {
			indirectType = indirectType.Elem()
		}
		fieldValue := reflect.New(indirectType).Interface()
		field := MetadataField{
			Code: fieldStruct.Name,
			Kind: fieldStruct.Type.Kind().String(),
		}
		switch field.Kind {
		case "bool":
			field.Type = "boolean"
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64":
			field.Type = "integer"
		case "float32", "float64":
			field.Type = "number"
		case "slice", "struct", "ptr":
			field.Type = "object"
		default:
			field.Type = field.Kind
		}
		if _, ok := fieldValue.(*time.Time); ok {
			field.Type = "string"
		}
		ref := fieldStruct.Tag.Get("ref")
		if ref != "" {
			fieldMeta := Meta(ref)
			if fieldMeta != nil {
				field.Type = fieldMeta.Name
				field.IsRef = true
				field.Value = fieldValue
				if indirectType.Kind() == reflect.Slice {
					field.IsArray = true
				}
			}
		}
		name := fieldStruct.Tag.Get("name")
		if name != "" {
			field.Name = name
		}
		if fieldStruct.Anonymous || indirectType.Kind() == reflect.Struct || indirectType.Kind() == reflect.Slice || indirectType.Kind() == reflect.Ptr {
			if fieldStruct.Name == "Model" || fieldStruct.Name == "ExtendField" {
				subMeta := parseMetadata(fieldValue)
				if subMeta != nil && len(subMeta.Fields) > 0 {
					if strings.HasPrefix(subMeta.FullName, "github.com/kuuland/kuu") {
						m.Fields = append(m.Fields, subMeta.Fields...)
					}
				}
				continue
			}
		}
		if field.Name != "" {
			m.Fields = append(m.Fields, field)
		}
	}
	return
}

// Meta
func Meta(value interface{}) (m *Metadata) {
	if v, ok := value.(string); ok {
		return metadata[v]
	} else {
		return parseMetadata(value)
	}
}

// Metalist
func Metalist() (arr []*Metadata) {
	for _, v := range metadata {
		arr = append(arr, v)
	}
	return
}

// RegisterMeta
func RegisterMeta() {
	tx := DB().Begin()
	tx = tx.Unscoped().Where(&Metadata{}).Delete(&Metadata{})
	for _, meta := range metadata {
		tx = tx.Create(meta)
	}
	if errs := tx.GetErrors(); len(errs) > 0 {
		ERROR(errs)
		if err := tx.Rollback(); err != nil {
			ERROR(err)
		}
	} else {
		if err := tx.Commit().Error; err != nil {
			ERROR(err)
		}
	}
}
