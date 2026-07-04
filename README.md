# wxbot-plugin-sdk-go

`wxbot-plugin-sdk-go` 是 wxbot / wechat-desk 的 Go 插件 SDK，用于开发独立进程插件。

SDK 提供三类能力：

- 插件协议类型：`Manifest`、`Event`、`Action`、`ActionResult`、`Permission`
- Process 插件运行器：自动暴露 `/health`、`/events`、`/action-results`、`/shutdown`
- 插件脚手架和打包工具：`wxbot-plugin init/build/package/validate/verify`

插件项目只需要依赖 SDK，不需要 import `wechat-desk/internal/...`。

## 安装 CLI

```bash
go install github.com/m9d2/wxbot-plugin-sdk-go/cmd/wxbot-plugin@latest
```

本地开发 SDK 时也可以直接运行：

```bash
go run ./cmd/wxbot-plugin validate -src ../your-plugin
```

## 创建插件项目

交互式创建：

```bash
wxbot-plugin init
```

执行后会逐步提示输入：

```text
创建 wxbot 插件项目
Plugin id [demo-plugin]:
Plugin name [demo-plugin]:
Version [1.0.0]:
Description [A wxbot process plugin.]:
Go module [github.com/your-org/demo-plugin]:
Output directory [./demo-plugin]:
Create frontend config page [Y/n]:
```

也可以像脚本一样非交互创建：

```bash
wxbot-plugin init \
  -yes \
  -id demo-plugin \
  -name 示例插件 \
  -version 1.0.0 \
  -description 收到 ping 后自动回复 pong \
  -module github.com/your-org/demo-plugin \
  -dir ./demo-plugin
```

生成后：

```bash
cd demo-plugin
go mod tidy
wxbot-plugin package -config wxbot-plugin.yaml
```

## 插件项目结构

推荐每个插件都是独立项目：

```text
your-plugin/
  go.mod
  manifest.json
  settings.schema.json
  wxbot-plugin.yaml
  .wxpluginignore
  process/
    main.go
  backend/
    bin/
  frontend/
    package.json
    src/
    dist/
  dist/
```

最小 `go.mod`：

```go
module github.com/your-org/your-plugin

go 1.23.0

require github.com/m9d2/wxbot-plugin-sdk-go v0.1.0
```

本地同时开发 SDK 和插件时，可以临时加：

```go
replace github.com/m9d2/wxbot-plugin-sdk-go => ../wxbot-plugin-sdk-go
```

## manifest.json

`manifest.json` 是插件包声明，安装、启动、权限展示都依赖它。

```json
{
  "id": "demo-plugin",
  "name": "示例插件",
  "version": "1.0.0",
  "runtime": "process",
  "description": "收到指定文字后自动回复。",
  "entry": "backend/bin/demo-plugin-process",
  "frontend": "frontend/dist/index.html",
  "settingsSchema": "settings.schema.json",
  "events": ["message.received"],
  "permissions": ["message:read", "message:send"]
}
```

字段说明：

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `id` | 是 | 插件唯一 ID，小写字母开头，支持数字、`_`、`-` |
| `name` | 是 | 插件名称 |
| `version` | 是 | 插件版本 |
| `runtime` | 是 | 当前推荐使用 `process` |
| `description` | 是 | 插件描述 |
| `entry` | process 必填 | 插件进程入口，相对于插件包根目录 |
| `frontend` | 否 | 插件配置页入口，相对于插件包根目录 |
| `settingsSchema` | 否 | 配置结构声明文件 |
| `events` | 否 | 插件订阅的事件；为空表示接收所有事件 |
| `permissions` | 否 | 插件声明的权限 |

## 后端插件

插件后端实现 `sdk.Plugin`：

```go
package main

import (
	"context"

	sdk "github.com/m9d2/wxbot-plugin-sdk-go"
)

type Plugin struct{}

func (p *Plugin) Manifest() sdk.Manifest {
	return sdk.Manifest{
		ID:          "demo-plugin",
		Name:        "示例插件",
		Version:     "1.0.0",
		Runtime:     sdk.RuntimeProcess,
		Description: "收到指定文字后自动回复。",
		Events:      []string{sdk.EventMessageReceived},
		Permissions: []sdk.Permission{
			sdk.PermissionMessageRead,
			sdk.PermissionMessageSend,
		},
	}
}

func (p *Plugin) OnEvent(ctx context.Context, event sdk.Event) ([]sdk.Action, error) {
	if event.Type != sdk.EventMessageReceived {
		return nil, nil
	}
	fromWxid := sdk.PayloadString(event.Payload, "fromWxid")
	content := sdk.PayloadString(event.Payload, "content")
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

func main() {
	if err := sdk.Run(&Plugin{}); err != nil {
		panic(err)
	}
}
```

`sdk.Run` 会自动启动 HTTP 服务，并暴露这些接口：

| 路径 | 说明 |
| --- | --- |
| `GET /health` | 健康检查 |
| `POST /events` | 接收 wechat-desk 推送的事件 |
| `POST /action-results` | 接收动作执行结果 |
| `POST /shutdown` | 主程序停止插件进程 |

wechat-desk 启动 process 插件时会注入环境变量：

| 环境变量 | 说明 |
| --- | --- |
| `WX_PLUGIN_ID` | 插件 ID |
| `WX_PLUGIN_VERSION` | 插件版本 |
| `WX_PLUGIN_PORT` | 插件监听端口 |
| `WX_PLUGIN_DATA_DIR` | 插件数据目录 |

读取插件数据目录：

```go
dataDir := sdk.DataDir("data")
```

## 插件自己的 API

如果插件配置页需要调用插件后端接口，实现 `sdk.RouteRegistrar`：

```go
type Plugin struct {
	store *Store
}

func (p *Plugin) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/settings", p.settings)
}

func (p *Plugin) settings(w http.ResponseWriter, r *http.Request) {
	userID := sdk.UserIDFromRequest(r)
	wxid := sdk.AccountWxidFromRequest(r)
	if userID == 0 || wxid == "" {
		sdk.Error(w, http.StatusBadRequest, "缺少账号上下文")
		return
	}
	sdk.WriteJSON(w, map[string]any{"success": true})
}
```

配置页请求路径仍然走 wechat-desk 网关：

```text
/api/plugins/{pluginId}/api/settings
```

wechat-desk 会代理到插件进程：

```text
插件进程 /api/settings
```

代理时会附加请求头：

| 请求头 | 说明 |
| --- | --- |
| `X-Wxbot-User-ID` | 当前系统用户 ID |
| `X-Wxbot-Account-Wxid` | 当前微信账号 wxid |
| `X-Wxbot-Plugin-ID` | 当前插件 ID |

## 事件

常用事件常量：

| 常量 | 事件名 | 说明 |
| --- | --- | --- |
| `sdk.EventMessageReceived` | `message.received` | 收到消息 |
| `sdk.EventMessageSent` | `message.sent` | 发送消息 |
| `sdk.EventGroupMemberJoined` | `group.member_joined` | 群成员入群 |
| `sdk.EventGroupMemberLeft` | `group.member_left` | 群成员退群 |
| `sdk.EventGroupMembersSynced` | `group.members_synced` | 群成员同步完成 |
| `sdk.EventScheduledTaskRun` | `scheduled_task.run` | 定时任务执行 |
| `sdk.EventPaymentRedPacket` | `payment.red_packet` | 红包事件 |
| `sdk.EventPaymentTransfer` | `payment.transfer` | 转账事件 |
| `sdk.EventContactAdded` | `contact.added` | 联系人新增 |

事件结构：

```go
type Event struct {
	Type        string
	UserID      int
	AccountWxid string
	Payload     map[string]any
	OccurredAt  int64
}
```

读取 payload 建议使用 SDK helper：

```go
fromWxid := sdk.PayloadString(event.Payload, "fromWxid")
messageID := sdk.PayloadInt(event.Payload, "messageId")
```

## 动作

插件不要直接调用 wechat-desk 数据库。需要让主程序执行能力时，返回 `sdk.Action`。

常用动作：

| Helper | 动作名 | 说明 |
| --- | --- | --- |
| `sdk.SendTextMessage` | `send_message` | 发送文本消息 |
| `sdk.AddFriend` | `add_friend` | 添加好友 |
| `sdk.UpdateRemark` | `update_remark` | 修改备注 |
| `sdk.SetLabel` | `set_label` | 设置标签 |

示例：

```go
action := sdk.SendTextMessage(event.AccountWxid, fromWxid, "hello")
action.Metadata = map[string]any{
	"bizId": 123,
}
return []sdk.Action{action}, nil
```

动作执行完成后，wechat-desk 会回调：

```go
func (p *Plugin) OnActionResult(ctx context.Context, result sdk.ActionResult) error {
	if !result.Success {
		// 可以在这里恢复业务状态、记录失败原因
		return nil
	}
	return nil
}
```

## 前端配置页

插件可以自带独立前端项目，构建产物放到 `frontend/dist`。

`manifest.json` 中配置：

```json
{
  "frontend": "frontend/dist/index.html"
}
```

前端请求插件 API：

```ts
await fetch(`/api/plugins/demo-plugin/api/settings?wxid=${wxid}`, {
  headers: {
    Authorization: `Bearer ${token}`,
    Accept: 'application/json'
  }
})
```

配置页通常由 wechat-desk 用 iframe 加载。建议插件前端监听宿主传入的 token / wxid，再请求插件 API。

## wxbot-plugin.yaml

SDK CLI 使用 `wxbot-plugin.yaml` 构建和打包：

```yaml
plugin:
  manifest: manifest.json
backend:
  type: go
  main: ./process
  output: ./backend/bin/demo-plugin-process
frontend:
  path: ./frontend
  build: npm install && npm run build
  dist: ./frontend/dist
package:
  output: ./dist/demo-plugin-1.0.0.wxplugin
```

## .wxpluginignore

`.wxpluginignore` 用于排除源码、依赖和临时产物：

```gitignore
process/
frontend/src/
frontend/node_modules/
frontend/package-lock.json
frontend/index.html
frontend/package.json
frontend/tsconfig.json
frontend/vite.config.ts
go.mod
go.sum
wxbot-plugin.yaml
README.md
dist/
```

最终插件包建议只包含：

```text
manifest.json
settings.schema.json
backend/bin/demo-plugin-process
frontend/dist/index.html
frontend/dist/assets/*
checksums.json
```

## 构建和打包

完整构建并打包：

```bash
wxbot-plugin package -config wxbot-plugin.yaml
```

只构建：

```bash
wxbot-plugin build -config wxbot-plugin.yaml
```

只校验插件目录：

```bash
wxbot-plugin validate -src .
```

校验已生成的插件包：

```bash
wxbot-plugin verify -file ./dist/demo-plugin-1.0.0.wxplugin
```

跳过构建直接打包：

```bash
wxbot-plugin package -config wxbot-plugin.yaml -skip-build
```

## 发布到插件后台

如果插件后台提供版本上传接口，可以使用：

```bash
wxbot-plugin publish \
  -file ./dist/demo-plugin-1.0.0.wxplugin \
  -url http://127.0.0.1:3200 \
  -token your-admin-token
```

## 开发建议

- 插件业务数据放在 `WX_PLUGIN_DATA_DIR` 下，不要写入 wechat-desk 数据库。
- 插件只通过事件和动作与主程序交互。
- 插件配置页只调用 `/api/plugins/{pluginId}/api/*`，不要直接请求插件进程端口。
- `Manifest()` 中声明的版本要和 `manifest.json` 保持一致。
- 业务动作失败时，在 `OnActionResult` 里回滚插件自己的业务状态。
- 插件包里不要包含源码、`node_modules`、`.git`、本地配置文件。

## 参考项目

发卡插件示例项目：

```text
card-sender-plugin/
```

它演示了：

- 独立 Go process 插件
- 插件自带 React 配置页
- 插件自己的本地 JSON 数据存储
- 文字触发后返回 `send_message` 动作
- 使用 `wxbot-plugin package` 生成 `.wxplugin`
