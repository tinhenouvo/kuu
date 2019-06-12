package kuu

import (
	"context"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/jtolds/gls"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"
)

var (
	// PrisDescKey
	PrisDescKey = "PrisDesc"
	// SignInfoKey
	SignInfoKey = "SignInfo"
	// IgnoreAuthKey
	IgnoreAuthKey = "IgnoreAuth"
	// UserMenus
	UserMenusKey = "UserMenus"
	// ValuesKey
	ValuesKey = "Values"
)

// StartTime
var StartTime time.Time

// HandlerFunc defines the handler used by ok middleware as return value.
type HandlerFunc func(*Context)

// HandlersChain defines a HandlerFunc array.
type HandlersChain []HandlerFunc

// RouteInfo represents a request route's specification which contains method and path and its handler.
type RouteInfo struct {
	Method       string
	Path         string
	HandlerFunc  HandlerFunc
	IgnorePrefix bool
}

// RoutesInfo defines a RouteInfo array.
type RoutesInfo []RouteInfo

// Engine
type Engine struct {
	*gin.Engine
}

// Values
type Values map[string]interface{}

// Default
func Default() (e *Engine) {
	e = &Engine{Engine: gin.Default()}
	onInit(e)
	return
}

// New
func New() (e *Engine) {
	e = &Engine{Engine: gin.New()}
	onInit(e)
	return
}

// SetValues
func SetValues(values gls.Values, call func()) {
	mgr.SetValues(values, call)
}

// GetValue
func GetValue(key interface{}) (value interface{}, ok bool) {
	return mgr.GetValue(key)
}

func shutdown(srv *http.Server) {
	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can"t be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	INFO("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		FATAL("Server Shutdown:", err)
	}
	// catching ctx.Done(). timeout of 5 seconds.
	select {
	case <-ctx.Done():
		INFO("timeout of 5 seconds.")
	}
	Release()
	INFO("Server exiting")
}

// RegisterWhitelist
func (e *Engine) RegisterWhitelist(rules ...interface{}) {
	AddWhitelist(rules...)
}

// Run
func (e *Engine) Run(addr ...string) {
	StartTime = time.Now()
	address := resolveAddress(addr)
	srv := &http.Server{
		Addr:    address,
		Handler: e.Engine,
	}
	go func() {
		INFO("Listening and serving HTTP on %s", address)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			FATAL("listen: %s", err)
		}
	}()
	shutdown(srv)
}

// RunTLS
func (e *Engine) RunTLS(addr, certFile, keyFile string) {
	StartTime = time.Now()
	srv := &http.Server{
		Addr:    addr,
		Handler: e.Engine,
	}
	go func() {
		INFO("Listening and serving HTTP on %s", addr)
		if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			FATAL("listen: %s", err)
		}
	}()
	shutdown(srv)
}

// RESTful
func (e *Engine) RESTful(values ...interface{}) {
	if len(values) == 0 {
		return
	}
	for _, v := range values {
		if v != nil {
			RESTful(e, v)
		}
	}
}

// ConvertKuuHandlers
var ConvertKuuHandlers = func(chain HandlersChain) (handlers gin.HandlersChain) {
	handlers = make(gin.HandlersChain, len(chain))
	for index, handler := range chain {
		handlers[index] = func(c *gin.Context) {
			vals := make(Values)
			kc := &Context{
				Context: c,
				Values:  &vals,
			}
			if !InWhitelist(c) {
				sign := GetSignContext(c)
				desc := GetPrivilegesDesc(c)
				kc.SignInfo = sign
				kc.PrisDesc = desc
			}
			glsVals := make(gls.Values)
			glsVals[SignInfoKey] = kc.SignInfo
			glsVals[PrisDescKey] = kc.PrisDesc
			glsVals[ValuesKey] = kc.Values
			SetValues(glsVals, func() { handler(kc) })
		}
	}
	return
}

// Overrite r.Group
func (e *Engine) Group(relativePath string, handlers ...HandlerFunc) *gin.RouterGroup {
	return e.Engine.Group(relativePath, ConvertKuuHandlers(handlers)...)
}

// Overrite r.Handle
func (e *Engine) Handle(httpMethod, relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return e.Engine.Handle(httpMethod, relativePath, ConvertKuuHandlers(handlers)...)
}

// Overrite r.POST
func (e *Engine) POST(relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return e.Engine.POST(relativePath, ConvertKuuHandlers(handlers)...)
}

// Overrite r.GET
func (e *Engine) GET(relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return e.Engine.GET(relativePath, ConvertKuuHandlers(handlers)...)
}

// Overrite r.DELETE
func (e *Engine) DELETE(relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return e.Engine.DELETE(relativePath, ConvertKuuHandlers(handlers)...)
}

// Overrite r.PATCH
func (e *Engine) PATCH(relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return e.Engine.PATCH(relativePath, ConvertKuuHandlers(handlers)...)
}

// Overrite r.PUT
func (e *Engine) PUT(relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return e.Engine.PUT(relativePath, ConvertKuuHandlers(handlers)...)
}

// Overrite r.OPTIONS
func (e *Engine) OPTIONS(relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return e.Engine.OPTIONS(relativePath, ConvertKuuHandlers(handlers)...)
}

// Overrite r.HEAD
func (e *Engine) HEAD(relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return e.Engine.HEAD(relativePath, ConvertKuuHandlers(handlers)...)
}

// Overrite r.Any
func (e *Engine) Any(relativePath string, handlers ...HandlerFunc) gin.IRoutes {
	return e.Engine.Any(relativePath, ConvertKuuHandlers(handlers)...)
}

// GetRoutinePrivilegesDesc
func GetRoutinePrivilegesDesc() *PrivilegesDesc {
	raw, _ := GetValue(PrisDescKey)
	if !IsBlank(raw) {
		desc := raw.(*PrivilegesDesc)
		if desc.IsValid() {
			return desc
		}
	}
	return nil
}

// GetRoutineValues
func GetRoutineValues() Values {
	raw, _ := GetValue(ValuesKey)
	if !IsBlank(raw) {
		values := *(raw.(*Values))
		return values
	}
	return nil
}

func (e *Engine) initConfigs() {
	if _, exists := C().Get("cors"); exists {
		if C().GetBool("cors") {
			config := cors.DefaultConfig()
			config.AllowAllOrigins = true
			config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "api_key", "Authorization", TokenKey, ""}
			e.Use(cors.New(config))
		} else {
			var config cors.Config
			C().GetInterface("cors", &config)
			e.Use(cors.New(config))
		}
	}

	if _, exists := C().Get("gzip"); exists {
		if C().GetBool("gzip") {
			e.Use(gzip.Gzip(gzip.DefaultCompression))
		} else {
			v := C().GetInt("gzip")
			if v != 0 {
				e.Use(gzip.Gzip(v))
			}
		}
	}
}

func (e *Engine) initStatics() {
	statics, exists := C().Get("statics")
	if !exists {
		return
	}
	if m, ok := statics.(map[string]interface{}); ok {
		for key, val := range m {
			str, ok := val.(string)
			if !ok {
				continue
			}
			stat, err := os.Lstat(str)
			if err != nil {
				ERROR("Static failed: %s", err.Error())
				continue
			}
			if stat.IsDir() {
				e.Static(key, str)
			} else {
				e.StaticFile(key, str)
			}
			AddWhitelist(regexp.MustCompile(fmt.Sprintf(`^GET\s%s`, key)))
		}
	}
}

func resolveAddress(addr []string) string {
	switch len(addr) {
	case 0:
		return ":8080"
	case 1:
		return addr[0]
	default:
		panic("too much parameters")
	}
}

func connectedPrint(name, args string) {
	INFO("%-8s is connected: %s", name, args)
}

func onInit(e *Engine) {
	initDataSources()
	initRedis()
	e.initConfigs()
	e.initStatics()

	// Register default callbacks
	registerCallbacks()
}
