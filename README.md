# wxbot-plugin-sdk-go

Go SDK and packaging CLI for wxbot process plugins.

## Plugin Backend

```go
package main

import (
	"context"

	sdk "github.com/m9d2/wxbot-plugin-sdk-go"
)

type Plugin struct{}

func (p *Plugin) Manifest() sdk.Manifest {
	return sdk.Manifest{
		ID:      "demo",
		Name:    "Demo",
		Version: "1.0.0",
		Runtime: sdk.RuntimeProcess,
		Events:  []string{sdk.EventMessageReceived},
	}
}

func (p *Plugin) OnEvent(ctx context.Context, event sdk.Event) ([]sdk.Action, error) {
	return []sdk.Action{
		sdk.SendTextMessage(event.AccountWxid, sdk.PayloadString(event.Payload, "fromWxid"), "hello"),
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

## CLI

```bash
go install github.com/m9d2/wxbot-plugin-sdk-go/cmd/wxbot-plugin@latest

wxbot-plugin build -config wxbot-plugin.yaml
wxbot-plugin package -config wxbot-plugin.yaml
wxbot-plugin validate -src .
wxbot-plugin verify -file ./dist/demo-1.0.0.wxplugin
```
