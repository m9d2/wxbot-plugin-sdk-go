package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
	files, err := pluginTemplateFiles(opts)
	if err != nil {
		return err
	}
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
