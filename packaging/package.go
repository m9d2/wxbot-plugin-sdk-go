package packaging

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	sdk "github.com/m9d2/wxbot-plugin-sdk-go"
)

const (
	ManifestFile  = "manifest.json"
	ChecksumsFile = "checksums.json"
	IgnoreFile    = ".wxpluginignore"
)

var packageIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,63}$`)

type Manifest struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Version        string           `json:"version"`
	Runtime        string           `json:"runtime"`
	Category       string           `json:"category,omitempty"`
	Summary        string           `json:"summary,omitempty"`
	Description    string           `json:"description"`
	Entry          string           `json:"entry,omitempty"`
	Frontend       string           `json:"frontend,omitempty"`
	Events         []string         `json:"events,omitempty"`
	Permissions    []sdk.Permission `json:"permissions,omitempty"`
	SettingsSchema string           `json:"settingsSchema,omitempty"`
	Icon           string           `json:"icon,omitempty"`
	Previews       []string         `json:"previews,omitempty"`
}

type Checksums struct {
	Algorithm string            `json:"algorithm"`
	Files     map[string]string `json:"files"`
}

type Package struct {
	Manifest Manifest
	Checksum string
	Files    map[string]string
}

func ReadPackage(path string) (Package, error) {
	archiveChecksum, err := fileSHA256(path)
	if err != nil {
		return Package{}, err
	}
	reader, err := zip.OpenReader(path)
	if err != nil {
		return Package{}, err
	}
	defer reader.Close()

	files := map[string]*zip.File{}
	hashes := map[string]string{}
	var manifest Manifest
	var checksums Checksums
	for _, file := range reader.File {
		name, err := cleanZipPath(file.Name)
		if err != nil {
			return Package{}, err
		}
		if file.FileInfo().IsDir() {
			continue
		}
		files[name] = file
		hash, err := zipFileSHA256(file)
		if err != nil {
			return Package{}, err
		}
		hashes[name] = hash
		switch name {
		case ManifestFile:
			if err := readZipJSON(file, &manifest); err != nil {
				return Package{}, fmt.Errorf("read manifest: %w", err)
			}
		case ChecksumsFile:
			if err := readZipJSON(file, &checksums); err != nil {
				return Package{}, fmt.Errorf("read checksums: %w", err)
			}
		}
	}
	if _, ok := files[ManifestFile]; !ok {
		return Package{}, errors.New("缺少 manifest.json")
	}
	if err := ValidateManifest(manifest, fileExistsMap(files)); err != nil {
		return Package{}, err
	}
	if checksums.Algorithm != "" {
		if err := verifyChecksums(checksums, hashes); err != nil {
			return Package{}, err
		}
	}
	return Package{Manifest: manifest, Checksum: archiveChecksum, Files: hashes}, nil
}

func ValidateDir(sourceDir string) (Manifest, error) {
	manifestPath := filepath.Join(sourceDir, ManifestFile)
	manifestRaw, err := os.ReadFile(manifestPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("读取 manifest.json 失败: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestRaw, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("manifest.json 格式错误: %w", err)
	}
	paths, err := collectFiles(sourceDir)
	if err != nil {
		return Manifest{}, err
	}
	exists := func(path string) bool {
		clean, err := cleanZipPath(path)
		if err != nil {
			return false
		}
		_, ok := paths[clean]
		return ok
	}
	if err := ValidateManifest(manifest, exists); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func ValidateManifest(manifest Manifest, exists func(string) bool) error {
	manifest.ID = strings.TrimSpace(manifest.ID)
	if !packageIDPattern.MatchString(manifest.ID) {
		return errors.New("manifest.id 格式错误")
	}
	if strings.TrimSpace(manifest.Name) == "" {
		return errors.New("manifest.name 不能为空")
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return errors.New("manifest.version 不能为空")
	}
	switch strings.TrimSpace(manifest.Runtime) {
	case sdk.RuntimeBuiltin, sdk.RuntimeDeclarative, sdk.RuntimeWebhook, sdk.RuntimeGoPlugin, sdk.RuntimeProcess:
	default:
		return errors.New("manifest.runtime 不支持")
	}
	for _, path := range append([]string{manifest.Entry, manifest.Frontend, manifest.SettingsSchema, manifest.Icon}, manifest.Previews...) {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if _, err := cleanZipPath(path); err != nil {
			return fmt.Errorf("manifest 路径无效: %w", err)
		}
		if exists != nil && !exists(path) {
			return fmt.Errorf("manifest 引用文件不存在: %s", path)
		}
	}
	switch manifest.Runtime {
	case sdk.RuntimeDeclarative, sdk.RuntimeGoPlugin, sdk.RuntimeProcess:
		if strings.TrimSpace(manifest.Entry) == "" {
			return fmt.Errorf("%s 插件必须配置 entry", manifest.Runtime)
		}
	}
	return nil
}

func PackDir(sourceDir, outputPath string) error {
	if _, err := ValidateDir(sourceDir); err != nil {
		return err
	}
	paths, err := collectFiles(sourceDir)
	if err != nil {
		return err
	}
	checksums, err := buildChecksums(paths)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	defer zw.Close()

	names := make([]string, 0, len(paths))
	for name := range paths {
		if name == ChecksumsFile {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := addFileToZip(zw, paths[name], name); err != nil {
			return err
		}
	}
	rawChecksums, err := json.MarshalIndent(checksums, "", "  ")
	if err != nil {
		return err
	}
	w, err := zw.Create(ChecksumsFile)
	if err != nil {
		return err
	}
	_, err = w.Write(rawChecksums)
	return err
}

func cleanZipPath(path string) (string, error) {
	path = strings.TrimSpace(filepath.ToSlash(path))
	path = strings.TrimPrefix(path, "/")
	if path == "" || strings.Contains(path, "\x00") {
		return "", errors.New("空路径")
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." || filepath.IsAbs(clean) {
		return "", fmt.Errorf("非法路径 %q", path)
	}
	return clean, nil
}

func fileExistsMap(files map[string]*zip.File) func(string) bool {
	return func(path string) bool {
		clean, err := cleanZipPath(path)
		if err != nil {
			return false
		}
		_, ok := files[clean]
		return ok
	}
}

func readZipJSON(file *zip.File, out any) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	return json.NewDecoder(rc).Decode(out)
}

func zipFileSHA256(file *zip.File) (string, error) {
	rc, err := file.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, rc); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func verifyChecksums(checksums Checksums, actual map[string]string) error {
	if strings.ToLower(checksums.Algorithm) != "sha256" {
		return errors.New("checksums.json 只支持 sha256")
	}
	for path, expected := range checksums.Files {
		clean, err := cleanZipPath(path)
		if err != nil {
			return err
		}
		if clean == ChecksumsFile {
			continue
		}
		if actual[clean] != strings.ToLower(strings.TrimSpace(expected)) {
			return fmt.Errorf("文件校验失败: %s", clean)
		}
	}
	return nil
}

func collectFiles(sourceDir string) (map[string]string, error) {
	ignorePatterns, err := loadIgnorePatterns(filepath.Join(sourceDir, IgnoreFile))
	if err != nil {
		return nil, err
	}
	result := map[string]string{}
	err = filepath.WalkDir(sourceDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		clean, err := cleanZipPath(rel)
		if err != nil {
			if entry.IsDir() {
				return nil
			}
			return err
		}
		if clean == "." || clean == IgnoreFile {
			return nil
		}
		if ignoredByPatterns(clean, ignorePatterns) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		result[clean] = path
		return nil
	})
	return result, err
}

func loadIgnorePatterns(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	patterns := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, filepath.ToSlash(line))
	}
	return patterns, nil
}

func ignoredByPatterns(path string, patterns []string) bool {
	path = filepath.ToSlash(path)
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(filepath.ToSlash(pattern))
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, "/") {
			prefix := strings.TrimSuffix(pattern, "/")
			if path == prefix || strings.HasPrefix(path, prefix+"/") {
				return true
			}
			continue
		}
		if ok, _ := filepath.Match(pattern, path); ok {
			return true
		}
		if path == pattern {
			return true
		}
	}
	return false
}

func buildChecksums(paths map[string]string) (Checksums, error) {
	result := Checksums{Algorithm: "sha256", Files: map[string]string{}}
	for name, path := range paths {
		if name == ChecksumsFile {
			continue
		}
		hash, err := fileSHA256(path)
		if err != nil {
			return Checksums{}, err
		}
		result.Files[name] = hash
	}
	return result, nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func addFileToZip(zw *zip.Writer, sourcePath, zipPath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = zipPath
	header.Method = zip.Deflate
	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(w, file)
	return err
}
