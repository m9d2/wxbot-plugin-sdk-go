package main

import (
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
		return fmt.Errorf("用法: wxbot-plugin build|package|pack|validate|verify|publish")
	}
	switch args[0] {
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
