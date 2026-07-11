package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultDevPort      = 3300
	defaultFrontendPort = 3301
	defaultBackendPort  = 49152
)

type devManifest struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

func runDev(args []string) error {
	fs := flag.NewFlagSet("dev", flag.ContinueOnError)
	configPath := fs.String("config", "wxbot-plugin.yaml", "plugin build config")
	host := fs.String("host", "", "development host")
	port := fs.Int("port", 0, "development host port")
	backendPort := fs.Int("backend-port", 0, "plugin backend port")
	frontendPort := fs.Int("frontend-port", 0, "frontend dev server port")
	accountWxid := fs.String("account", "", "mock account wxid")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, baseDir, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	manifest, err := loadDevManifest(baseDir, cfg.Plugin.Manifest)
	if err != nil {
		return err
	}
	applyDevDefaults(&cfg.Dev)
	if *host != "" {
		cfg.Dev.Host = *host
	}
	if *port != 0 {
		cfg.Dev.Port = *port
	}
	if *backendPort != 0 {
		cfg.Dev.BackendPort = *backendPort
	}
	if *frontendPort != 0 {
		cfg.Dev.FrontendPort = *frontendPort
	}
	if *accountWxid != "" {
		cfg.Dev.Accounts[0].Wxid = *accountWxid
	}
	if err := validateDevConfig(cfg.Dev); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	processes := make(chan error, 2)
	if strings.TrimSpace(cfg.Backend.Type) != "" {
		command := cfg.Dev.BackendCommand
		if command == "" {
			command = "go run " + shellQuote(cfg.Backend.Main)
		}
		env := append(os.Environ(),
			"WX_PLUGIN_ID="+manifest.ID,
			"WX_PLUGIN_VERSION="+manifest.Version,
			"WX_PLUGIN_PORT="+strconv.Itoa(cfg.Dev.BackendPort),
			"WX_PLUGIN_DATA_DIR="+filepath.Join(baseDir, ".wxbot-plugin", "data"),
		)
		if err := startDevCommand(ctx, baseDir, command, env, processes); err != nil {
			return fmt.Errorf("启动插件后端失败: %w", err)
		}
	}
	if strings.TrimSpace(cfg.Frontend.Path) == "" {
		return errors.New("frontend.path 不能为空")
	}
	frontendDir := absPath(baseDir, cfg.Frontend.Path)
	command := cfg.Dev.FrontendCommand
	if command == "" {
		command = fmt.Sprintf("npm run dev -- --host %s --port %d --strictPort", shellQuote(cfg.Dev.Host), cfg.Dev.FrontendPort)
	}
	if err := startDevCommand(ctx, frontendDir, command, os.Environ(), processes); err != nil {
		return fmt.Errorf("启动前端失败: %w", err)
	}

	handler, err := newDevHandler(manifest, cfg.Dev)
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr:    net.JoinHostPort(cfg.Dev.Host, strconv.Itoa(cfg.Dev.Port)),
		Handler: handler,
	}
	serverResult := make(chan error, 1)
	go func() {
		serverResult <- server.ListenAndServe()
	}()

	fmt.Printf("插件开发环境已启动: http://%s\n", server.Addr)
	fmt.Printf("Mock 微信实例: %s (%s)\n", cfg.Dev.Accounts[0].NickName, cfg.Dev.Accounts[0].Wxid)

	select {
	case <-ctx.Done():
		return server.Shutdown(context.Background())
	case err := <-processes:
		stop()
		_ = server.Shutdown(context.Background())
		if err == nil {
			return errors.New("开发进程已退出")
		}
		return err
	case err := <-serverResult:
		stop()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func applyDevDefaults(cfg *DevConfig) {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = defaultDevPort
	}
	if cfg.BackendPort == 0 {
		cfg.BackendPort = defaultBackendPort
	}
	if cfg.FrontendPort == 0 {
		cfg.FrontendPort = defaultFrontendPort
	}
	if cfg.UserID == 0 {
		cfg.UserID = 1
	}
	if len(cfg.Accounts) == 0 {
		cfg.Accounts = []DevAccount{{
			Wxid:     "wxid_dev",
			NickName: "本地开发实例",
			Status:   "online",
		}}
	}
	for i := range cfg.Accounts {
		if cfg.Accounts[i].Status == "" {
			cfg.Accounts[i].Status = "online"
		}
	}
}

func validateDevConfig(cfg DevConfig) error {
	for name, port := range map[string]int{
		"dev.port":         cfg.Port,
		"dev.backendPort":  cfg.BackendPort,
		"dev.frontendPort": cfg.FrontendPort,
	} {
		if port < 1 || port > 65535 {
			return fmt.Errorf("%s 必须在 1-65535 之间", name)
		}
	}
	if strings.TrimSpace(cfg.Accounts[0].Wxid) == "" {
		return errors.New("dev.accounts[0].wxid 不能为空")
	}
	return nil
}

func loadDevManifest(baseDir, path string) (devManifest, error) {
	if strings.TrimSpace(path) == "" {
		path = "manifest.json"
	}
	raw, err := os.ReadFile(absPath(baseDir, path))
	if err != nil {
		return devManifest{}, fmt.Errorf("读取 manifest 失败: %w", err)
	}
	var manifest devManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return devManifest{}, fmt.Errorf("manifest 格式错误: %w", err)
	}
	if strings.TrimSpace(manifest.ID) == "" {
		return devManifest{}, errors.New("manifest.id 不能为空")
	}
	return manifest, nil
}

func startDevCommand(ctx context.Context, dir, command string, env []string, result chan<- error) error {
	var cmd *exec.Cmd
	if isWindows() {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		err := cmd.Wait()
		if ctx.Err() == nil {
			if err == nil {
				result <- fmt.Errorf("%s: 进程已退出", command)
			} else {
				result <- fmt.Errorf("%s: %w", command, err)
			}
		}
	}()
	return nil
}

func newDevHandler(manifest devManifest, cfg DevConfig) (http.Handler, error) {
	frontendURL, err := url.Parse(fmt.Sprintf("http://%s:%d", cfg.Host, cfg.FrontendPort))
	if err != nil {
		return nil, err
	}
	backendURL, err := url.Parse(fmt.Sprintf("http://%s:%d", cfg.Host, cfg.BackendPort))
	if err != nil {
		return nil, err
	}
	frontendProxy := httputil.NewSingleHostReverseProxy(frontendURL)
	backendProxy := httputil.NewSingleHostReverseProxy(backendURL)
	apiPrefix := "/api/plugins/" + manifest.ID

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			writeDevHost(w, manifest, cfg.Accounts[0])
		case r.URL.Path == apiPrefix+"/sdk/accounts":
			writeDevJSON(w, map[string]any{"success": true, "accounts": cfg.Accounts})
		case r.URL.Path == apiPrefix || strings.HasPrefix(r.URL.Path, apiPrefix+"/"):
			r.URL.Path = strings.TrimPrefix(r.URL.Path, apiPrefix)
			if r.URL.Path == "" {
				r.URL.Path = "/"
			}
			r.Header.Set("X-Wxbot-User-ID", strconv.Itoa(cfg.UserID))
			r.Header.Set("X-Wxbot-Account-Wxid", requestAccountWxid(r, cfg.Accounts[0].Wxid))
			r.Header.Set("X-Wxbot-Plugin-ID", manifest.ID)
			backendProxy.ServeHTTP(w, r)
		case strings.HasPrefix(r.URL.Path, "/__wxbot_plugin/"):
			r.URL.Path = strings.TrimPrefix(r.URL.Path, "/__wxbot_plugin")
			frontendProxy.ServeHTTP(w, r)
		default:
			frontendProxy.ServeHTTP(w, r)
		}
	}), nil
}

func requestAccountWxid(r *http.Request, fallback string) string {
	if wxid := strings.TrimSpace(r.URL.Query().Get("wxid")); wxid != "" {
		return wxid
	}
	if r.Body == nil || r.Header.Get("Content-Type") != "application/json" {
		return fallback
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return fallback
	}
	r.Body = io.NopCloser(strings.NewReader(string(raw)))
	var body struct {
		Wxid string `json:"wxid"`
	}
	if json.Unmarshal(raw, &body) == nil && strings.TrimSpace(body.Wxid) != "" {
		return strings.TrimSpace(body.Wxid)
	}
	return fallback
}

func writeDevHost(w http.ResponseWriter, manifest devManifest, account DevAccount) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data, _ := json.Marshal(map[string]string{
		"pluginId": manifest.ID,
		"wxid":     account.Wxid,
		"token":    "dev-token",
	})
	_, _ = io.WriteString(w, `<!doctype html><html><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Plugin Dev Host</title><style>html,body,iframe{width:100%;height:100%;margin:0;border:0;display:block}</style></head><body><iframe id="plugin" src="/__wxbot_plugin/"></iframe><script>
const context = `+string(data)+`;
const frame = document.getElementById('plugin');
function initialize() {
  frame.contentWindow.postMessage({type:'wxbot-plugin:init', ...context}, '*');
  frame.contentWindow.postMessage({type:'wxbot-plugin-context', ...context}, '*');
}
window.addEventListener('message', event => {
  if (event.data?.type === 'wxbot-plugin:ready' || event.data?.type === 'wxbot-plugin-ready') initialize();
});
frame.addEventListener('load', () => { initialize(); setTimeout(initialize, 100); });
</script></body></html>`)
}

func writeDevJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func shellQuote(value string) string {
	if isWindows() {
		return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func isWindows() bool {
	return os.PathSeparator == '\\'
}
