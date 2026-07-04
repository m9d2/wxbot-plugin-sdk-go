package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	EnvPluginID      = "WX_PLUGIN_ID"
	EnvPluginVersion = "WX_PLUGIN_VERSION"
	EnvPluginPort    = "WX_PLUGIN_PORT"
	EnvPluginDataDir = "WX_PLUGIN_DATA_DIR"
)

type RouteRegistrar interface {
	RegisterRoutes(*http.ServeMux)
}

type ServerOption func(*serverOptions)

type serverOptions struct {
	addr     string
	dataDir  string
	logger   *log.Logger
	shutdown func()
}

func WithAddr(addr string) ServerOption {
	return func(opts *serverOptions) {
		opts.addr = strings.TrimSpace(addr)
	}
}

func WithDataDir(dataDir string) ServerOption {
	return func(opts *serverOptions) {
		opts.dataDir = strings.TrimSpace(dataDir)
	}
}

func WithLogger(logger *log.Logger) ServerOption {
	return func(opts *serverOptions) {
		opts.logger = logger
	}
}

func DataDir(fallback string) string {
	if value := strings.TrimSpace(os.Getenv(EnvPluginDataDir)); value != "" {
		return value
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return "data"
}

func Run(plugin Plugin, options ...ServerOption) error {
	if plugin == nil {
		return errors.New("plugin is nil")
	}
	opts := serverOptions{
		addr:    defaultAddr(),
		dataDir: DataDir("data"),
		logger:  log.New(os.Stdout, "", log.LstdFlags),
	}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}
	if opts.logger == nil {
		opts.logger = log.New(os.Stdout, "", log.LstdFlags)
	}
	if err := os.MkdirAll(opts.dataDir, 0o755); err != nil {
		return err
	}
	mux := http.NewServeMux()
	server := &http.Server{Addr: opts.addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, map[string]any{"success": true})
	})
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var event Event
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		if err := decoder.Decode(&event); err != nil {
			Error(w, http.StatusBadRequest, "事件格式错误")
			return
		}
		actions, err := plugin.OnEvent(r.Context(), event)
		if err != nil {
			Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		WriteJSON(w, map[string]any{"success": true, "actions": actions})
	})
	mux.HandleFunc("/action-results", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var result ActionResult
		decoder := json.NewDecoder(r.Body)
		decoder.UseNumber()
		if err := decoder.Decode(&result); err != nil {
			Error(w, http.StatusBadRequest, "动作结果格式错误")
			return
		}
		if err := plugin.OnActionResult(r.Context(), result); err != nil {
			Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		WriteJSON(w, map[string]any{"success": true})
	})
	mux.HandleFunc("/shutdown", func(w http.ResponseWriter, _ *http.Request) {
		WriteJSON(w, map[string]any{"success": true})
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
		}()
	})
	if registrar, ok := plugin.(RouteRegistrar); ok {
		registrar.RegisterRoutes(mux)
	}
	manifest := plugin.Manifest()
	opts.logger.Printf("%s process plugin listening on %s", manifest.ID, opts.addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func UserIDFromRequest(r *http.Request) int {
	value, _ := strconv.Atoi(strings.TrimSpace(r.Header.Get("X-Wxbot-User-ID")))
	return value
}

func AccountWxidFromRequest(r *http.Request) string {
	if wxid := strings.TrimSpace(r.Header.Get("X-Wxbot-Account-Wxid")); wxid != "" {
		return wxid
	}
	if wxid := strings.TrimSpace(r.URL.Query().Get("wxid")); wxid != "" {
		return wxid
	}
	return ""
}

func WriteJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func Error(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	WriteJSON(w, map[string]any{"success": false, "message": message})
}

func defaultAddr() string {
	port := strings.TrimSpace(os.Getenv(EnvPluginPort))
	if port == "" {
		port = "49152"
	}
	return "127.0.0.1:" + port
}
