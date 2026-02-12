package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config 支持从 JSON 配置文件 + 环境变量加载
type Config struct {
	// 多模态模型列表
	Models []string `json:"models"`
	// SiliconFlow OpenAI 兼容网关
	SiliconflowBaseURL string `json:"siliconflow_base_url"`
	// API Key（仅从 config.json 读取，勿提交到仓库）
	SiliconflowAPIKey string `json:"siliconflow_api_key"`
	// HTML 模板路径
	TemplatePath string `json:"template_path"`
	// 邀请码 SQLite 路径
	SQLitePath string `json:"sqlite_path"`
	// 管理后台鉴权 Token
	AdminToken string `json:"admin_token"`
}

func defaultConfig() Config {
	return Config{
		Models:             []string{"Qwen/Qwen3-VL-32B-Instruct"},
		SiliconflowBaseURL: "https://api.siliconflow.cn",
		TemplatePath:       "web/result.html",
		SQLitePath:         "data/invites.db",
	}
}

func loadConfigFile(path string) (Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	var c Config
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// mergeEnv 覆盖来自环境变量的配置
func mergeEnv(c Config) Config {
	if env := strings.TrimSpace(os.Getenv("VISION_MODELS")); env != "" {
		c.Models = splitCSV(env)
	}
	if env := strings.TrimSpace(os.Getenv("SILICONFLOW_BASEURL")); env != "" {
		c.SiliconflowBaseURL = env
	}
	if env := strings.TrimSpace(os.Getenv("TEMPLATE_PATH")); env != "" {
		c.TemplatePath = env
	}
	if env := strings.TrimSpace(os.Getenv("SQLITE_PATH")); env != "" {
		c.SQLitePath = env
	}
	if env := strings.TrimSpace(os.Getenv("ADMIN_TOKEN")); env != "" {
		c.AdminToken = env
	}
	return c
}

// loadConfig 先读取 JSON 文件（默认 ./config.json，可用 SERVER_CONFIG 指定），再用环境变量覆盖
func loadConfig() Config {
	path := resolveConfigPath()
	c := defaultConfig()
	if b, err := os.Stat(path); err == nil && !b.IsDir() {
		if fileCfg, err2 := loadConfigFile(path); err2 == nil {
			// 合并：文件覆盖默认
			if len(fileCfg.Models) > 0 {
				c.Models = fileCfg.Models
			}
			if fileCfg.SiliconflowBaseURL != "" {
				c.SiliconflowBaseURL = fileCfg.SiliconflowBaseURL
			}
			if fileCfg.SiliconflowAPIKey != "" {
				c.SiliconflowAPIKey = fileCfg.SiliconflowAPIKey
			}
			if fileCfg.TemplatePath != "" {
				c.TemplatePath = fileCfg.TemplatePath
			}
			if fileCfg.SQLitePath != "" {
				c.SQLitePath = fileCfg.SQLitePath
			}
			if fileCfg.AdminToken != "" {
				c.AdminToken = fileCfg.AdminToken
			}
		} else {
			fmt.Fprintf(os.Stderr, "warn: read config file failed: %v\n", err2)
		}
	}
	c = mergeEnv(c)
	// 若模板为相对路径，则相对于配置文件所在目录进行解析，便于二进制在仓库根或其他目录运行
	if !filepath.IsAbs(c.TemplatePath) {
		c.TemplatePath = filepath.Join(filepath.Dir(path), c.TemplatePath)
	}
	// 启动日志：打印实际使用的配置路径与关键项（API Key 打码）
	masked := c.SiliconflowAPIKey
	if len(masked) > 8 {
		masked = masked[:4] + "***" + masked[len(masked)-3:]
	}
	adminMasked := ""
	if c.AdminToken != "" {
		adminMasked = "set"
	} else {
		adminMasked = "empty"
	}
	fmt.Fprintf(os.Stderr, "using config: %s\nmodels=%v baseURL=%s key=%s template=%s sqlite=%s admin=%s\n",
		path, c.Models, c.SiliconflowBaseURL, masked, c.TemplatePath, c.SQLitePath, adminMasked)
	return c
}

// resolveConfigPath 按优先级解析配置路径：
// 1) SERVER_CONFIG 指定的文件；
// 2) 工作目录下 config.json；
// 3) 可执行文件所在目录下 config.json；
// 4) 工作目录下 screensot-server/config.json（兼容从仓库根运行）。
func resolveConfigPath() string {
	if p := strings.TrimSpace(os.Getenv("SERVER_CONFIG")); p != "" {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	candidates := []string{"config.json"}
	if exe, err := os.Executable(); err == nil && exe != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "config.json"))
	}
	candidates = append(candidates, filepath.Join("screensot-server", "config.json"))
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return "config.json"
}
