package kuu

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin/binding"
	"github.com/jinzhu/gorm"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// RestDesc
type RestDesc struct {
	Create bool
	Delete bool
	Query  bool
	Update bool
}

// IsValid
func (r *RestDesc) IsValid() bool {
	return r.Create || r.Delete || r.Query || r.Update
}

// RESTful
func RESTful(r *Engine, value interface{}) (desc *RestDesc) {
	desc = new(RestDesc)
	// Scope value can't be nil
	if value == nil {
		PANIC("Model can't be nil")
	}
	reflectType := reflect.ValueOf(value).Type()
	for reflectType.Kind() == reflect.Slice || reflectType.Kind() == reflect.Ptr {
		reflectType = reflectType.Elem()
	}

	// Scope value need to be a struct
	if reflectType.Kind() != reflect.Struct {
		PANIC("Model need to be a struct")
	}

	structName := reflectType.Name()
	routePrefix := C().GetString("prefix")
	routePath := fmt.Sprintf("%s/%s", routePrefix, strings.ToLower(structName))

	// Get all fields
	for i := 0; i < reflectType.NumField(); i++ {
		fieldStruct := reflectType.Field(i)
		if strings.Contains(string(fieldStruct.Tag), "rest") {
			// mounted RESTful routes
			tagSettings := parseTagSetting(fieldStruct.Tag, "rest")
			routeAlias := strings.TrimSpace(fieldStruct.Tag.Get("route"))
			if routeAlias != "" {
				routePath = fmt.Sprintf("%s/%s", routePrefix, strings.ToLower(routeAlias))
			}
			var (
				createMethod = "POST"
				deleteMethod = "DELETE"
				queryMethod  = "GET"
				updateMethod = "PUT"
			)
			for key, val := range tagSettings {
				key = strings.TrimSpace(strings.ToUpper(key))
				val = strings.TrimSpace(strings.ToUpper(val))
				switch key {
				case "C", "CREATE":
					createMethod = val
				case "D", "DELETE", "REMOVE":
					deleteMethod = val
				case "R", "READ", "QUERY", "FIND":
					queryMethod = val
				case "U", "UPDATE":
					updateMethod = val
				}
			}

			if _, exists := tagSettings["-"]; exists {
				createMethod = "-"
				deleteMethod = "-"
				queryMethod = "-"
				updateMethod = "-"
			}

			// Method conflict
			if methodConflict([]string{createMethod, deleteMethod, queryMethod, updateMethod}) {
				ERROR("restful routes method conflict:\n%s\n%s\n%s\n%s\n ",
					fmt.Sprintf(" - create %s: %-8s %s", structName, createMethod, routePath),
					fmt.Sprintf(" - delete %s: %-8s %s", structName, deleteMethod, routePath),
					fmt.Sprintf(" - update %s: %-8s %s", structName, updateMethod, routePath),
					fmt.Sprintf(" - query  %s: %-8s %s", structName, queryMethod, routePath),
				)
			} else {
				if createMethod != "-" {
					desc.Create = true
					r.Handle(createMethod, routePath, func(c *Context) {
						var (
							docs  []interface{}
							multi bool
							err   error
						)
						// 事务执行
						err = c.WithTransaction(func(tx *gorm.DB) *gorm.DB {
							var body interface{}
							if err := c.ShouldBindBodyWith(&body, binding.JSON); err != nil {
								_ = tx.AddError(errors.New("解析请求体失败"))
								return tx
							}
							indirectScopeValue := indirect(reflect.ValueOf(body))
							if indirectScopeValue.Kind() == reflect.Slice {
								multi = true
								for i := 0; i < indirectScopeValue.Len(); i++ {
									doc := reflect.New(reflectType).Interface()
									if err := Copy(indirectScopeValue.Index(i).Interface(), doc); err != nil {
										_ = tx.AddError(err)
										return tx
									}
									docs = append(docs, doc)
								}
							} else {
								doc := reflect.New(reflectType).Interface()
								if err := Copy(body, doc); err != nil {
									_ = tx.AddError(err)
									return tx
								}
								docs = append(docs, doc)
							}
							for _, doc := range docs {
								tx = tx.Create(doc)
							}
							return tx
						})
						// 响应结果
						if err != nil {
							c.STDErr("新增失败", err)
						} else {
							if multi {
								c.STD(docs)
							} else {
								c.STD(docs[0])
							}
						}
					})
				}
				if deleteMethod != "-" {
					desc.Delete = true
					r.Handle(deleteMethod, routePath, func(c *Context) {
						var (
							result interface{}
							err    error
						)
						// 事务执行
						err = c.WithTransaction(func(tx *gorm.DB) *gorm.DB {
							var params struct {
								All    bool
								Multi  bool
								UnSoft bool
								Cond   map[string]interface{}
							}
							if c.Query("cond") != "" {
								var retCond map[string]interface{}
								Parse(c.Query("cond"), &retCond)
								params.Cond = retCond

								if c.Query("multi") != "" || c.Query("all") != "" {
									params.Multi = true
								}
								if c.Query("unsoft") != "" {
									params.UnSoft = true
								}
							} else {
								if err := c.ShouldBindBodyWith(&params, binding.JSON); err != nil {
									_ = tx.AddError(errors.New("解析请求体失败"))
									return tx
								}
							}
							if IsBlank(params.Cond) {
								_ = tx.AddError(errors.New("删除条件不能为空"))
								return tx
							}
							var multi bool
							if params.Multi || params.All {
								multi = true
							}
							params.Cond = underlineMap(params.Cond)
							for key, val := range params.Cond {
								if m, ok := val.(map[string]interface{}); ok {
									query, args := fieldQuery(m, key)
									if query != "" && len(args) > 0 {
										tx = tx.Where(query, args...)
										delete(params.Cond, key)
									}
								}
							}
							if !IsBlank(params.Cond) {
								query := reflect.New(reflectType).Interface()
								if err := Copy(params.Cond, query); err != nil {
									_ = tx.AddError(err)
									return tx
								}
								tx = tx.Where(query)
							}
							if multi {
								result = reflect.New(reflect.SliceOf(reflectType)).Interface()
								tx = tx.Find(result)
								indirectValue := indirect(reflect.ValueOf(result))
								if indirectValue.Len() > 0 {
									result = indirectValue.Index(i).Addr().Interface()
								}
								if params.UnSoft {
									tx = tx.Unscoped()
								}
								tx = tx.Delete(reflect.New(reflectType).Interface())
							} else {
								result = reflect.New(reflectType).Interface()
								tx = tx.First(result)
								if params.UnSoft {
									tx = tx.Unscoped()
								}
								tx = tx.Delete(reflect.New(reflectType).Interface())
							}
							return tx
						})
						// 响应结果
						if err != nil {
							c.STDErr("删除失败", err)
						} else {
							c.STD(result)
						}
					})
				}
				if queryMethod != "-" {
					desc.Query = true
					r.Handle(queryMethod, routePath, func(c *Context) {
						ret := map[string]interface{}{}
						scope := DB().NewScope(value)
						// 处理cond
						var cond map[string]interface{}
						rawCond := c.Query("cond")
						if rawCond != "" {
							Parse(rawCond, &cond)
							var retCond map[string]interface{}
							Parse(rawCond, &retCond)
							ret["cond"] = retCond
						}
						db := DB().Model(reflect.New(reflectType).Interface())
						ms := db.NewScope(reflect.New(reflectType).Interface())
						if cond != nil {
							for key, val := range cond {
								if key == "$and" || key == "$or" {
									// arr = [{"pass":"123"},{"pass":{"$regex":"^333"}}]
									if arr, ok := val.([]interface{}); ok {
										queries := make([]string, 0)
										args := make([]interface{}, 0)
										for _, item := range arr {
											// obj = {"pass":"123"}
											obj, ok := item.(map[string]interface{})
											if !ok {
												continue
											}
											for k, v := range obj {
												if m, ok := v.(map[string]interface{}); ok {
													q, a := fieldQuery(m, k)
													if !IsBlank(q) && !IsBlank(a) {
														queries = append(queries, q)
														args = append(args, a...)
													}
												} else {
													queries = append(queries, fmt.Sprintf("\"%s\" = ?", k))
													args = append(args, v)
												}
											}
										}
										if !IsBlank(queries) && !IsBlank(args) {
											if key == "$or" {
												db = db.Where(strings.Join(queries, " OR "), args...)
											} else {
												db = db.Where(strings.Join(queries, " AND "), args...)
											}
										}
									}
									delete(cond, key)
								} else if m, ok := val.(map[string]interface{}); ok {
									query, args := fieldQuery(m, key)
									if query != "" && len(args) > 0 {
										db = db.Where(query, args...)
										delete(cond, key)
									}
								} else {
									if field, ok := ms.FieldByName(key); ok {
										db = db.Where(fmt.Sprintf("%s = ?", field.DBName), val)
									} else {
										ERROR("字段不存在：%s", key)
									}
								}
							}
						}
						// 处理project
						rawProject := c.Query("project")
						if rawProject != "" {
							split := strings.Split(rawProject, ",")
							var (
								retProject []string
								columns    []string
							)
							for _, name := range split {
								if strings.HasPrefix(name, "-") {
									name = name[1:]
								}
								if field, ok := scope.FieldByName(name); ok {
									columns = append(columns, field.DBName)
									retProject = append(retProject, field.Name)
								}
							}
							db = db.Select(columns)
							ret["project"] = strings.Join(retProject, ",")
						}
						// 处理sort
						rawSort := c.Query("sort")
						if rawSort != "" {
							split := strings.Split(rawSort, ",")
							var retSort []string
							for _, name := range split {
								direction := "asc"
								if strings.HasPrefix(name, "-") {
									name = name[1:]
									direction = "desc"
								}
								if field, ok := scope.FieldByName(name); ok {
									db = db.Order(fmt.Sprintf("%s %s", field.DBName, direction))
									if direction == "desc" {
										retSort = append(retSort, "-"+field.Name)
									} else {
										retSort = append(retSort, field.Name)
									}
								}
							}
							ret["sort"] = strings.Join(retSort, ",")
						}
						// 处理preload
						rawPreload := c.Query("preload")
						if rawPreload != "" {
							ms := db.NewScope(reflect.New(reflectType).Interface())
							split := strings.Split(rawPreload, ",")
							for _, item := range split {
								if v, ok := ms.FieldByName(item); ok {
									db = db.Preload(v.Name)
								}
							}
							ret["preload"] = rawPreload
						}

						list := reflect.New(reflect.SliceOf(reflectType)).Interface()
						// 调用RestBeforeQuery钩子
						if h, ok := reflect.New(reflectType).Interface().(RestQueryHooks); ok {
							db = h.RestBeforeQuery(db, c)
						}
						countDB := db
						// 处理range
						rawRange := strings.ToUpper(c.DefaultQuery("range", "PAGE"))
						ret["range"] = rawRange
						// 处理page、size
						var (
							page int
							size int
						)
						rawPage := c.DefaultQuery("page", "1")
						rawSize := c.DefaultQuery("size", "30")
						if v, err := strconv.Atoi(rawPage); err == nil {
							page = v
						}
						if v, err := strconv.Atoi(rawSize); err == nil {
							size = v
						}
						if rawRange == "PAGE" {
							db = db.Offset((page - 1) * size).Limit(size)
							ret["page"] = page
							ret["size"] = size
						}
						db = db.Find(list)
						ret["list"] = OmitPassword(value, list)
						// 处理totalrecords、totalpages
						var totalRecords int
						countDB = countDB.Count(&totalRecords)
						ret["totalrecords"] = totalRecords
						if rawRange == "PAGE" {
							ret["totalpages"] = int(math.Ceil(float64(totalRecords) / float64(size)))
						}
						c.STD(ret)
					})
				}
				if updateMethod != "-" {
					desc.Update = true
					r.Handle(updateMethod, routePath, func(c *Context) {
						var (
							result interface{}
							err    error
						)
						// 事务执行
						err = c.WithTransaction(func(tx *gorm.DB) *gorm.DB {
							var params struct {
								All   bool
								Multi bool
								Cond  map[string]interface{}
								Doc   map[string]interface{}
								Auto  bool
							}
							if err := c.ShouldBindBodyWith(&params, binding.JSON); err != nil {
								_ = tx.AddError(errors.New("解析请求体失败"))
								return tx
							}
							if IsBlank(params.Cond) || IsBlank(params.Doc) {
								_ = tx.AddError(errors.New("更新条件和内容不能为空"))
								return tx
							}
							var multi bool
							if params.Multi || params.All {
								multi = true
							}
							if IsBlank(params.Cond) && !multi {
								_ = tx.AddError(errors.New("必须指定批量更新标记"))
								return tx
							}
							// 处理更新条件
							for key, val := range params.Cond {
								if m, ok := val.(map[string]interface{}); ok {
									query, args := fieldQuery(m, key)
									if query != "" && len(args) > 0 {
										tx = tx.Where(query, args...)
										delete(params.Cond, key)
									}
								}
							}
							if !IsBlank(params.Cond) {
								q := reflect.New(reflectType).Interface()
								if err := Copy(params.Cond, q); err != nil {
									_ = tx.AddError(err)
									return tx
								}
								tx = tx.Where(q)
							}
							// 先查询更新前的数据
							if multi {
								result = reflect.New(reflect.SliceOf(reflectType)).Interface()
								tx = tx.Find(result)
							} else {
								result = reflect.New(reflectType).Interface()
								tx = tx.First(result)
							}

							updateFields := func(val interface{}) error {
								doc := reflect.New(reflectType).Interface()
								if err := Copy(params.Doc, doc); err != nil {
									return err
								}
								rawScope := tx.NewScope(val)
								docScope := tx.NewScope(doc)
								if params.Auto {
									for key, val := range params.Doc {
										if field, ok := rawScope.FieldByName(key); ok {
											fieldKind := field.Field.Kind()
											if fieldKind == reflect.Interface || fieldKind == reflect.Slice || fieldKind == reflect.Array || fieldKind == reflect.Struct {
												docField, _ := docScope.FieldByName(key)
												docFieldValue := docField.Field.Interface()
												if err := field.Set(docFieldValue); err != nil {
													return err
												}
											} else {
												if err := rawScope.SetColumn(key, val); err != nil {
													return err
												}
											}
										}
									}
									tx.Save(val)
								} else {
									tx = tx.Model(val).Updates(doc)
								}
								return tx.Error
							}
							if indirectScopeValue := indirect(reflect.ValueOf(result)); indirectScopeValue.Kind() == reflect.Slice {
								for i := 0; i < indirectScopeValue.Len(); i++ {
									item := indirectScopeValue.Index(i).Interface()
									if err := updateFields(item); err != nil {
										_ = tx.AddError(err)
										return tx
									}
								}
							} else {
								if err := updateFields(result); err != nil {
									_ = tx.AddError(err)
									return tx
								}
							}
							return tx
						})
						// 响应结果
						if err != nil {
							c.STDErr("修改失败", err)
						} else {
							c.STD(result)
						}
					})
				}
			}
			break
		}
	}
	return
}

func underlineMap(m map[string]interface{}) map[string]interface{} {
	for k, v := range m {
		delete(m, k)
		m[underline(k)] = v
	}
	return m
}

func underline(str string) string {
	reg := regexp.MustCompile(`([a-z])([A-Z])`)
	return strings.ToLower(reg.ReplaceAllString(str, "${1}_${2}"))
}

func fieldQuery(m map[string]interface{}, key string) (query string, args []interface{}) {
	key = underline(key)
	if raw, has := m["$regex"]; has {
		keyword := raw.(string)
		hasPrefix := strings.HasPrefix(keyword, "^")
		hasSuffix := strings.HasSuffix(keyword, "$")
		if !hasPrefix && !hasSuffix {
			keyword = fmt.Sprintf("^%s$", keyword)
			hasPrefix = true
			hasSuffix = true
		}
		if hasPrefix {
			keyword = keyword[1:]
		}
		if hasSuffix {
			keyword = keyword[:len(keyword)-1]
		}
		a := make([]string, 0)
		if hasPrefix {
			a = append(a, "%")
		}
		a = append(a, keyword)
		if hasSuffix {
			a = append(a, "%")
		}
		keyword = strings.Join(a, "")
		return fmt.Sprintf(" %s LIKE ?", key), []interface{}{keyword}
	} else if raw, has := m["$in"]; has {
		return fmt.Sprintf("%s IN (?)", key), []interface{}{raw}
	} else if raw, has := m["$nin"]; has {
		return fmt.Sprintf("%s NOT IN (?)", key), []interface{}{raw}
	} else if raw, has := m["$eq"]; has {
		return fmt.Sprintf("%s = ?", key), []interface{}{raw}
	} else if raw, has := m["$ne"]; has {
		return fmt.Sprintf("%s <> ?", key), []interface{}{raw}
	} else if raw, has := m["$exists"]; has {
		return fmt.Sprintf("%s IS NOT NULL", key), []interface{}{raw}
	} else {
		gt, hgt := m["$gt"]
		gte, hgte := m["$gte"]
		lt, hlt := m["$lt"]
		lte, hlte := m["$lte"]
		if hgt {
			if hlt {
				return fmt.Sprintf("%s > ? AND %s < ?", key, key), []interface{}{gt, lt}
			} else if hlte {
				return fmt.Sprintf("%s > ? AND %s <= ?", key, key), []interface{}{gt, lte}
			} else {
				return fmt.Sprintf("%s > ?", key), []interface{}{gt}
			}
		} else if hgte {
			if hlt {
				return fmt.Sprintf("%s >= ? AND %s < ?", key, key), []interface{}{gte, lt}
			} else if hlte {
				return fmt.Sprintf("%s >= ? AND %s <= ?", key, key), []interface{}{gte, lte}
			} else {
				return fmt.Sprintf("%s >= ?", key), []interface{}{gte}
			}
		} else if hlt {
			return fmt.Sprintf("%s < ?", key), []interface{}{lt}
		} else if hlte {
			return fmt.Sprintf("%s <= ?", key), []interface{}{lte}
		}
	}
	return
}

func indirect(reflectValue reflect.Value) reflect.Value {
	for reflectValue.Kind() == reflect.Ptr {
		reflectValue = reflectValue.Elem()
	}
	return reflectValue
}

func methodConflict(arr []string) bool {
	for i, s := range arr {
		if s == "-" {
			continue
		}
		for j, t := range arr {
			if t == "-" || j == i {
				continue
			}
			if s == t {
				return true
			}
		}
	}
	return false
}

func parseTagSetting(tags reflect.StructTag, tagKey string) map[string]string {
	setting := map[string]string{}
	str := tags.Get(tagKey)
	split := strings.Split(str, ";")
	for _, value := range split {
		v := strings.Split(value, ":")
		k := strings.TrimSpace(strings.ToUpper(v[0]))
		if len(v) >= 2 {
			setting[k] = strings.Join(v[1:], ":")
		} else {
			setting[k] = k
		}
	}
	return setting
}
