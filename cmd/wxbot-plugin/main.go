package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/m9d2/wxbot-plugin-sdk-go/packaging"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Plugin struct {
		Manifest string `yaml:"manifest"`
	} `yaml:"plugin"`
	Backend struct {
		Type   string `yaml:"type"`
		Main   string `yaml:"main"`
		Output string `yaml:"output"`
	} `yaml:"backend"`
	Frontend struct {
		Path  string `yaml:"path"`
		Build string `yaml:"build"`
		Dist  string `yaml:"dist"`
	} `yaml:"frontend"`
	Package struct {
		Output string `yaml:"output"`
	} `yaml:"package"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("用法: wxbot-plugin init|build|package|pack|validate|verify|publish")
	}
	switch args[0] {
	case "init":
		return runInit(args[1:])
	case "build":
		return runBuild(args[1:])
	case "package":
		return runPackage(args[1:])
	case "pack":
		return runPack(args[1:])
	case "validate":
		return runValidate(args[1:])
	case "verify":
		return runVerify(args[1:])
	case "publish":
		return runPublish(args[1:])
	default:
		return fmt.Errorf("未知命令: %s", args[0])
	}
}

type initOptions struct {
	ID          string
	Name        string
	Version     string
	Description string
	Module      string
	Dir         string
	Frontend    bool
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	id := fs.String("id", "", "plugin id")
	name := fs.String("name", "", "plugin display name")
	version := fs.String("version", "1.0.0", "plugin version")
	description := fs.String("description", "", "plugin description")
	module := fs.String("module", "", "go module path")
	dir := fs.String("dir", "", "output directory")
	frontend := fs.Bool("frontend", true, "create frontend template")
	yes := fs.Bool("yes", false, "use flags/defaults without prompts")
	if err := fs.Parse(args); err != nil {
		return err
	}
	opts := initOptions{
		ID:          strings.TrimSpace(*id),
		Name:        strings.TrimSpace(*name),
		Version:     strings.TrimSpace(*version),
		Description: strings.TrimSpace(*description),
		Module:      strings.TrimSpace(*module),
		Dir:         strings.TrimSpace(*dir),
		Frontend:    *frontend,
	}
	if !*yes {
		reader := bufio.NewReader(os.Stdin)
		var err error
		opts, err = promptInitOptions(reader, opts)
		if err != nil {
			return err
		}
	}
	return createPluginProject(opts)
}

func promptInitOptions(reader *bufio.Reader, opts initOptions) (initOptions, error) {
	fmt.Println("创建 wxbot 插件项目")
	var err error
	opts.ID, err = promptString(reader, "Plugin id", opts.ID, "demo-plugin")
	if err != nil {
		return opts, err
	}
	opts.Name, err = promptString(reader, "Plugin name", opts.Name, opts.ID)
	if err != nil {
		return opts, err
	}
	opts.Version, err = promptString(reader, "Version", opts.Version, "1.0.0")
	if err != nil {
		return opts, err
	}
	opts.Description, err = promptString(reader, "Description", opts.Description, "A wxbot process plugin.")
	if err != nil {
		return opts, err
	}
	opts.Module, err = promptString(reader, "Go module", opts.Module, "github.com/your-org/"+opts.ID)
	if err != nil {
		return opts, err
	}
	opts.Dir, err = promptString(reader, "Output directory", opts.Dir, "./"+opts.ID)
	if err != nil {
		return opts, err
	}
	opts.Frontend, err = promptBool(reader, "Create frontend config page", opts.Frontend)
	if err != nil {
		return opts, err
	}
	return opts, nil
}

func promptString(reader *bufio.Reader, label, current, fallback string) (string, error) {
	defaultValue := strings.TrimSpace(current)
	if defaultValue == "" {
		defaultValue = fallback
	}
	fmt.Printf("%s [%s]: ", label, defaultValue)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(line)
	if value == "" {
		value = defaultValue
	}
	return value, nil
}

func promptBool(reader *bufio.Reader, label string, fallback bool) (bool, error) {
	defaultValue := "Y/n"
	if !fallback {
		defaultValue = "y/N"
	}
	fmt.Printf("%s [%s]: ", label, defaultValue)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	value := strings.ToLower(strings.TrimSpace(line))
	if value == "" {
		return fallback, nil
	}
	return value == "y" || value == "yes" || value == "true" || value == "1", nil
}

func createPluginProject(opts initOptions) error {
	if strings.TrimSpace(opts.ID) == "" {
		return fmt.Errorf("plugin id 不能为空")
	}
	if strings.TrimSpace(opts.Name) == "" {
		opts.Name = opts.ID
	}
	if strings.TrimSpace(opts.Version) == "" {
		opts.Version = "1.0.0"
	}
	if strings.TrimSpace(opts.Description) == "" {
		opts.Description = "A wxbot process plugin."
	}
	if strings.TrimSpace(opts.Module) == "" {
		opts.Module = "github.com/your-org/" + opts.ID
	}
	if strings.TrimSpace(opts.Dir) == "" {
		opts.Dir = opts.ID
	}
	if err := validateInitID(opts.ID); err != nil {
		return err
	}
	targetDir, err := filepath.Abs(opts.Dir)
	if err != nil {
		return err
	}
	if err := ensureEmptyDir(targetDir); err != nil {
		return err
	}
	files := pluginTemplateFiles(opts)
	for path, body := range files {
		target := filepath.Join(targetDir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
			return err
		}
	}
	for _, dir := range []string{"backend/bin", "dist"} {
		if err := os.MkdirAll(filepath.Join(targetDir, filepath.FromSlash(dir)), 0o755); err != nil {
			return err
		}
	}
	fmt.Printf("\n插件项目已生成: %s\n", targetDir)
	fmt.Println("\n下一步:")
	fmt.Printf("  cd %s\n", targetDir)
	fmt.Println("  go mod tidy")
	if opts.Frontend {
		fmt.Println("  npm --prefix frontend install")
	}
	fmt.Println("  wxbot-plugin package -config wxbot-plugin.yaml")
	return nil
}

func validateInitID(id string) error {
	if strings.ContainsAny(id, `/\`) || strings.HasPrefix(id, ".") || strings.TrimSpace(id) == "" {
		return fmt.Errorf("plugin id 格式错误")
	}
	for i, r := range id {
		if i == 0 && (r < 'a' || r > 'z') {
			return fmt.Errorf("plugin id 必须以小写字母开头")
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return fmt.Errorf("plugin id 只能包含小写字母、数字、-、_")
	}
	return nil
}

func ensureEmptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return os.MkdirAll(dir, 0o755)
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("目标目录不为空: %s", dir)
	}
	return nil
}

func pluginTemplateFiles(opts initOptions) map[string]string {
	files := map[string]string{
		"go.mod":               goModTemplate(opts),
		"manifest.json":        manifestTemplate(opts),
		"settings.schema.json": settingsSchemaTemplate(),
		"wxbot-plugin.yaml":    pluginConfigTemplate(opts),
		".wxpluginignore":      ignoreTemplate(opts),
		"README.md":            pluginReadmeTemplate(opts),
		"process/main.go":      processMainTemplate(opts),
	}
	if opts.Frontend {
		files["frontend/package.json"] = frontendPackageTemplate(opts)
		files["frontend/index.html"] = frontendIndexTemplate(opts)
		files["frontend/tsconfig.json"] = frontendTSConfigTemplate()
		files["frontend/vite.config.ts"] = frontendViteTemplate()
		files["frontend/src/main.tsx"] = frontendMainTemplate()
		files["frontend/src/App.tsx"] = frontendAppTemplate(opts)
		files["frontend/src/styles.css"] = frontendStylesTemplate()
	}
	return files
}

func goModTemplate(opts initOptions) string {
	return fmt.Sprintf(`module %s

go 1.23.0

require github.com/m9d2/wxbot-plugin-sdk-go v0.1.0
`, opts.Module)
}

func manifestTemplate(opts initOptions) string {
	frontendLine := ""
	if opts.Frontend {
		frontendLine = `  "frontend": "frontend/dist/index.html",` + "\n"
	}
	return fmt.Sprintf(`{
  "id": %q,
  "name": %q,
  "version": %q,
  "runtime": "process",
  "description": %q,
  "entry": "backend/bin/%s-process",
%s  "settingsSchema": "settings.schema.json",
  "events": [
    "message.received"
  ],
  "permissions": [
    "message:read",
    "message:send"
  ]
}
`, opts.ID, opts.Name, opts.Version, opts.Description, opts.ID, frontendLine)
}

func settingsSchemaTemplate() string {
	return `{
  "type": "object",
  "properties": {
    "enabled": {
      "type": "boolean",
      "title": "启用插件",
      "default": true
    },
    "replyText": {
      "type": "string",
      "title": "回复内容",
      "default": "hello"
    }
  }
}
`
}

func pluginConfigTemplate(opts initOptions) string {
	frontend := ""
	if opts.Frontend {
		frontend = `frontend:
  path: ./frontend
  build: npm install && npm run build
  dist: ./frontend/dist
`
	}
	return fmt.Sprintf(`plugin:
  manifest: manifest.json
backend:
  type: go
  main: ./process
  output: ./backend/bin/%s-process
%spackage:
  output: ./dist/%s-%s.wxplugin
`, opts.ID, frontend, opts.ID, opts.Version)
}

func ignoreTemplate(opts initOptions) string {
	lines := []string{
		"process/",
		"go.mod",
		"go.sum",
		"wxbot-plugin.yaml",
		"README.md",
		"dist/",
	}
	if opts.Frontend {
		lines = append(lines,
			"frontend/src/",
			"frontend/node_modules/",
			"frontend/package-lock.json",
			"frontend/index.html",
			"frontend/package.json",
			"frontend/tsconfig.json",
			"frontend/vite.config.ts",
		)
	}
	return strings.Join(lines, "\n") + "\n"
}

func pluginReadmeTemplate(opts initOptions) string {
	return fmt.Sprintf(strings.Join([]string{
		"# %s",
		"",
		"%s",
		"",
		"## 开发",
		"",
		"```bash",
		"go mod tidy",
		"```",
		"",
		"## 打包",
		"",
		"```bash",
		"wxbot-plugin package -config wxbot-plugin.yaml",
		"wxbot-plugin verify -file ./dist/%s-%s.wxplugin",
		"```",
		"",
		"## 说明",
		"",
		"当前插件是 process runtime。安装后 wechat-desk 会启动 `backend/bin/%s-process`，并把 `/api/plugins/%s/api/*` 代理到插件进程。",
		"",
	}, "\n"), opts.Name, opts.Description, opts.ID, opts.Version, opts.ID, opts.ID)
}

func processMainTemplate(opts initOptions) string {
	return fmt.Sprintf(`package main

import (
	"context"
	"net/http"
	"strings"

	sdk "github.com/m9d2/wxbot-plugin-sdk-go"
)

const pluginID = %q

type Plugin struct{}

func (p *Plugin) Manifest() sdk.Manifest {
	return sdk.Manifest{
		ID:          pluginID,
		Name:        %q,
		Version:     %q,
		Runtime:     sdk.RuntimeProcess,
		Description: %q,
		Events:      []string{sdk.EventMessageReceived},
		Permissions: []sdk.Permission{
			sdk.PermissionMessageRead,
			sdk.PermissionMessageSend,
		},
	}
}

func (p *Plugin) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/settings", p.settings)
}

func (p *Plugin) OnEvent(ctx context.Context, event sdk.Event) ([]sdk.Action, error) {
	if event.Type != sdk.EventMessageReceived {
		return nil, nil
	}
	fromWxid := sdk.PayloadString(event.Payload, "fromWxid")
	content := strings.TrimSpace(sdk.PayloadString(event.Payload, "content"))
	if fromWxid == "" || content != "ping" {
		return nil, nil
	}
	return []sdk.Action{
		sdk.SendTextMessage(event.AccountWxid, fromWxid, "pong"),
	}, nil
}

func (p *Plugin) OnActionResult(ctx context.Context, result sdk.ActionResult) error {
	return nil
}

func (p *Plugin) settings(w http.ResponseWriter, r *http.Request) {
	wxid := sdk.AccountWxidFromRequest(r)
	if wxid == "" {
		sdk.Error(w, http.StatusBadRequest, "缺少 wxid")
		return
	}
	sdk.WriteJSON(w, map[string]any{
		"success": true,
		"config": map[string]any{
			"enabled": true,
			"replyText": "pong",
		},
	})
}

func main() {
	if err := sdk.Run(&Plugin{}); err != nil {
		panic(err)
	}
}
`, opts.ID, opts.Name, opts.Version, opts.Description)
}

func frontendPackageTemplate(opts initOptions) string {
	return fmt.Sprintf(`{
  "name": "@wxbot-plugin/%s-frontend",
  "private": true,
  "version": %q,
  "type": "module",
  "scripts": {
    "build": "tsc -p tsconfig.json --noEmit && vite build --config vite.config.ts",
    "dev": "vite --config vite.config.ts --host 0.0.0.0 --port 3301"
  },
  "dependencies": {
    "@vitejs/plugin-react": "^4.3.4",
    "vite": "^5.4.19",
    "typescript": "^5.8.3",
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "@types/node": "^20.19.1",
    "@types/react": "^18.3.18",
    "@types/react-dom": "^18.3.5"
  }
}
`, opts.ID, opts.Version)
}

func frontendIndexTemplate(opts initOptions) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>%s</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
`, opts.Name)
}

func frontendTSConfigTemplate() string {
	return `{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["dom", "dom.iterable", "esnext"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "noEmit": true,
    "baseUrl": ".",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "isolatedModules": true,
    "resolveJsonModule": true,
    "sourceMap": true,
    "types": ["node", "react", "react-dom"]
  },
  "include": ["src/**/*", "vite.config.ts"],
  "exclude": ["dist"]
}
`
}

func frontendViteTemplate() string {
	return `import react from '@vitejs/plugin-react'

export default {
  base: './',
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:3100',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: true,
  },
}
`
}

func frontendMainTemplate() string {
	return `import React from 'react'
import {createRoot} from 'react-dom/client'
import './styles.css'
import {App} from './App'

createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
`
}

func frontendAppTemplate(opts initOptions) string {
	return fmt.Sprintf(`import {useEffect, useState} from 'react'

type HostContext = {
  token?: string
  wxid?: string
}

export function App() {
  const [context, setContext] = useState<HostContext>({})

  useEffect(() => {
    const handler = (event: MessageEvent) => {
      if (!event.data || typeof event.data !== 'object') return
      if (event.data.type !== 'wxbot-plugin-context') return
      setContext({
        token: event.data.token,
        wxid: event.data.wxid,
      })
    }
    window.addEventListener('message', handler)
    window.parent?.postMessage({type: 'wxbot-plugin-ready', pluginId: %q}, '*')
    return () => window.removeEventListener('message', handler)
  }, [])

  return (
    <main className="page">
      <section className="panel">
        <h1>%s</h1>
        <p>%s</p>
        <dl>
          <dt>当前 wxid</dt>
          <dd>{context.wxid || '未选择'}</dd>
        </dl>
      </section>
    </main>
  )
}
`, opts.ID, opts.Name, opts.Description)
}

func frontendStylesTemplate() string {
	return `:root {
  color: #17211c;
  background: #f6f7f4;
  font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
  min-width: 320px;
  min-height: 100vh;
  background: #f6f7f4;
}

.page {
  min-height: 100vh;
  padding: 24px;
}

.panel {
  max-width: 720px;
}

h1 {
  margin: 0 0 10px;
  font-size: 24px;
  font-weight: 700;
}

p {
  margin: 0 0 20px;
  color: #58635d;
}

dl {
  display: grid;
  grid-template-columns: 120px 1fr;
  gap: 8px 16px;
  margin: 0;
}

dt {
  color: #6b746f;
}

dd {
  margin: 0;
  color: #17211c;
}
`
}

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	configPath := fs.String("config", "wxbot-plugin.yaml", "plugin build config")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, baseDir, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	if err := buildBackend(cfg, baseDir); err != nil {
		return err
	}
	return buildFrontend(cfg, baseDir)
}

func runPackage(args []string) error {
	fs := flag.NewFlagSet("package", flag.ContinueOnError)
	configPath := fs.String("config", "wxbot-plugin.yaml", "plugin build config")
	skipBuild := fs.Bool("skip-build", false, "skip backend/frontend build")
	out := fs.String("out", "", "output .wxplugin package path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, baseDir, err := loadConfig(*configPath)
	if err != nil {
		return err
	}
	if !*skipBuild {
		if err := buildBackend(cfg, baseDir); err != nil {
			return err
		}
		if err := buildFrontend(cfg, baseDir); err != nil {
			return err
		}
	}
	manifest, err := packaging.ValidateDir(baseDir)
	if err != nil {
		return err
	}
	output := strings.TrimSpace(*out)
	if output == "" {
		output = strings.TrimSpace(cfg.Package.Output)
	}
	if output == "" {
		output = filepath.Join(baseDir, "dist", manifest.ID+"-"+manifest.Version+".wxplugin")
	}
	if !filepath.IsAbs(output) {
		output = filepath.Join(baseDir, output)
	}
	if err := packaging.PackDir(baseDir, output); err != nil {
		return err
	}
	fmt.Printf("插件包已生成: %s\n", output)
	return nil
}

func runPack(args []string) error {
	fs := flag.NewFlagSet("pack", flag.ContinueOnError)
	src := fs.String("src", ".", "plugin source directory")
	out := fs.String("out", "", "output .wxplugin package path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*out) == "" {
		return fmt.Errorf("缺少 -out")
	}
	if err := packaging.PackDir(*src, *out); err != nil {
		return err
	}
	abs, _ := filepath.Abs(*out)
	fmt.Printf("插件包已生成: %s\n", abs)
	return nil
}

func runValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	src := fs.String("src", ".", "plugin source directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	manifest, err := packaging.ValidateDir(*src)
	if err != nil {
		return err
	}
	fmt.Printf("插件目录校验通过: %s v%s runtime=%s\n", manifest.ID, manifest.Version, manifest.Runtime)
	return nil
}

func runVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	file := fs.String("file", "", "plugin package file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*file) == "" {
		return fmt.Errorf("缺少 -file")
	}
	pkg, err := packaging.ReadPackage(*file)
	if err != nil {
		return err
	}
	fmt.Printf("插件包校验通过: %s v%s runtime=%s checksum=%s\n", pkg.Manifest.ID, pkg.Manifest.Version, pkg.Manifest.Runtime, pkg.Checksum)
	return nil
}

func runPublish(args []string) error {
	fs := flag.NewFlagSet("publish", flag.ContinueOnError)
	file := fs.String("file", "", "plugin package file")
	url := fs.String("url", "", "plugin admin base url")
	token := fs.String("token", "", "plugin admin token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*file) == "" || strings.TrimSpace(*url) == "" {
		return fmt.Errorf("缺少 -file 或 -url")
	}
	pkg, err := packaging.ReadPackage(*file)
	if err != nil {
		return err
	}
	endpoint := strings.TrimRight(*url, "/") + "/api/admin/plugins/" + pkg.Manifest.ID + "/versions"
	raw, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if strings.TrimSpace(*token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(*token))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("发布失败: HTTP %d", resp.StatusCode)
	}
	fmt.Printf("插件版本已发布: %s v%s\n", pkg.Manifest.ID, pkg.Manifest.Version)
	return nil
}

func loadConfig(path string) (Config, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, "", err
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, "", err
	}
	baseDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return Config{}, "", err
	}
	return cfg, baseDir, nil
}

func buildBackend(cfg Config, baseDir string) error {
	if strings.TrimSpace(cfg.Backend.Type) == "" {
		return nil
	}
	if strings.TrimSpace(cfg.Backend.Type) != "go" {
		return fmt.Errorf("暂不支持后端类型: %s", cfg.Backend.Type)
	}
	mainPath := absPath(baseDir, cfg.Backend.Main)
	output := absPath(baseDir, cfg.Backend.Output)
	if strings.TrimSpace(mainPath) == "" || strings.TrimSpace(output) == "" {
		return fmt.Errorf("backend.main 和 backend.output 不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	return runCommand(baseDir, "go", "build", "-o", output, mainPath)
}

func buildFrontend(cfg Config, baseDir string) error {
	if strings.TrimSpace(cfg.Frontend.Build) == "" {
		return nil
	}
	frontendDir := absPath(baseDir, cfg.Frontend.Path)
	if strings.TrimSpace(frontendDir) == "" {
		frontendDir = baseDir
	}
	return runShell(frontendDir, cfg.Frontend.Build)
}

func absPath(baseDir, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) {
		return value
	}
	return filepath.Join(baseDir, value)
}

func runShell(dir, command string) error {
	if runtime.GOOS == "windows" {
		return runCommand(dir, "cmd", "/C", command)
	}
	return runCommand(dir, "sh", "-c", command)
}

func runCommand(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}
