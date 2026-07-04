package main

import (
	"fmt"
	"strings"
)

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
