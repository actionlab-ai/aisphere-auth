package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/actionlab-ai/aisphere-auth/internal/config"
	"github.com/actionlab-ai/aisphere-auth/internal/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	root := newRootCommand()
	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var configFile string
	var printConfig bool

	cmd := &cobra.Command{
		Use:           "aisphere-auth",
		Short:         "AI Sphere 统一认证与授权服务",
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: strings.TrimSpace(`AI Sphere Auth 是 AI Sphere 平台统一认证与授权服务。

它负责：
  1. 对接 Casdoor 登录与回调。
  2. 创建和管理 AI Sphere Session。
  3. 标准化 Principal 用户身份。
  4. 为 SkillHub、AgentRuntime、SQLHub 等业务服务提供 introspect / authz check。
  5. 支持 Redis Session、内部 Service Token、离线 .run 交付。`),
		RunE: func(cmd *cobra.Command, args []string) error {
			v, used, cfg, err := loadRuntimeConfig(cmd, configFile)
			if err != nil {
				return err
			}
			if printConfig {
				return printSafeConfig(cfg, used)
			}
			logConfigSource(used, v)
			return server.New(cfg).Run()
		},
	}

	cmd.SetHelpTemplate(chineseHelpTemplate())
	cmd.SetUsageTemplate(chineseUsageTemplate())
	cmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "配置文件路径，默认自动读取 ./configs/config.yaml 或 ./config.yaml")
	cmd.PersistentFlags().String("addr", "", "服务监听地址，例如 :18080；覆盖配置项 server.addr")
	cmd.PersistentFlags().String("mode", "", "Gin 运行模式：debug、release、test；覆盖配置项 server.mode")
	cmd.PersistentFlags().String("public-base-url", "", "外部访问基础地址，例如 https://auth.example.com；覆盖 server.publicBaseURL")
	cmd.PersistentFlags().String("session-provider", "", "Session 存储类型：memory 或 redis；覆盖 session.provider")
	cmd.PersistentFlags().String("redis-addrs", "", "Redis 地址，多个用英文逗号分隔，例如 127.0.0.1:6379；覆盖 session.redis.addrs")
	cmd.PersistentFlags().String("casdoor-endpoint", "", "Casdoor 服务地址，例如 http://127.0.0.1:8000；覆盖 casdoor.endpoint")
	cmd.PersistentFlags().String("casdoor-client-id", "", "Casdoor Application Client ID；覆盖 casdoor.clientId")
	cmd.PersistentFlags().String("casdoor-client-secret", "", "Casdoor Application Client Secret；覆盖 casdoor.clientSecret")
	cmd.PersistentFlags().String("casdoor-redirect-url", "", "Casdoor OAuth 回调地址；覆盖 casdoor.redirectURL")
	cmd.PersistentFlags().String("service-token", "", "内部服务调用凭证；覆盖 internal.serviceToken")
	cmd.PersistentFlags().Bool("service-token-required", false, "是否强制校验内部服务调用凭证；覆盖 internal.serviceTokenRequired")
	cmd.PersistentFlags().BoolVar(&printConfig, "print-config", false, "打印脱敏后的最终配置并退出")

	cmd.AddCommand(newCheckConfigCommand(&configFile))
	cmd.AddCommand(newVersionCommand())
	return cmd
}

func newCheckConfigCommand(configFile *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-config",
		Short: "检查配置文件是否可加载",
		Long:  "读取配置文件、环境变量和命令行参数，校验关键配置是否合法，但不启动服务。",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, used, cfg, err := loadRuntimeConfig(cmd, *configFile)
			if err != nil {
				return err
			}
			fmt.Println("配置检查通过")
			if used != "" {
				fmt.Printf("配置文件: %s\n", used)
			} else {
				fmt.Println("配置文件: 未使用，当前使用默认值 / 环境变量 / 命令行参数")
			}
			fmt.Printf("监听地址: %s\n", cfg.Server.Addr)
			fmt.Printf("Session Provider: %s\n", cfg.Session.Provider)
			fmt.Printf("Casdoor Endpoint: %s\n", cfg.Casdoor.Endpoint)
			return nil
		},
	}
	cmd.SetHelpTemplate(chineseHelpTemplate())
	cmd.SetUsageTemplate(chineseUsageTemplate())
	return cmd
}

func newVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("aisphere-auth\nversion: %s\ncommit: %s\nbuildDate: %s\n", version, commit, date)
		},
	}
	cmd.SetHelpTemplate(chineseHelpTemplate())
	cmd.SetUsageTemplate(chineseUsageTemplate())
	return cmd
}

func loadRuntimeConfig(cmd *cobra.Command, configFile string) (*viper.Viper, string, config.Config, error) {
	v := config.NewViper(configFile)
	bindFlags(cmd, v)
	used, err := config.ReadConfig(v)
	if err != nil {
		return nil, "", config.Config{}, err
	}
	cfg, err := config.Load(v)
	if err != nil {
		return nil, used, config.Config{}, err
	}
	return v, used, cfg, nil
}

func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	bindings := map[string]string{
		"addr":                   "server.addr",
		"mode":                   "server.mode",
		"public-base-url":        "server.publicBaseURL",
		"session-provider":       "session.provider",
		"redis-addrs":            "session.redis.addrs",
		"casdoor-endpoint":       "casdoor.endpoint",
		"casdoor-client-id":      "casdoor.clientId",
		"casdoor-client-secret":  "casdoor.clientSecret",
		"casdoor-redirect-url":   "casdoor.redirectURL",
		"service-token":          "internal.serviceToken",
		"service-token-required": "internal.serviceTokenRequired",
	}
	for flagName, key := range bindings {
		if flag := cmd.Flag(flagName); flag != nil {
			_ = v.BindPFlag(key, flag)
		}
	}
}

func printSafeConfig(cfg config.Config, configFile string) error {
	type safeConfig struct {
		ConfigFile string        `json:"configFile"`
		Config     config.Config `json:"config"`
	}
	cfg.Casdoor.ClientSecret = maskSecret(cfg.Casdoor.ClientSecret)
	cfg.Session.Redis.Password = maskSecret(cfg.Session.Redis.Password)
	cfg.Token.SigningSecret = maskSecret(cfg.Token.SigningSecret)
	cfg.Internal.ServiceToken = maskSecret(cfg.Internal.ServiceToken)
	payload, err := json.MarshalIndent(safeConfig{ConfigFile: configFile, Config: cfg}, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(payload))
	return nil
}

func maskSecret(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	if len(value) <= 6 {
		return "******"
	}
	return value[:3] + "******" + value[len(value)-3:]
}

func logConfigSource(configFile string, v *viper.Viper) {
	if configFile != "" {
		log.Printf("加载配置文件: %s", configFile)
		return
	}
	if v.ConfigFileUsed() == "" {
		log.Printf("未找到配置文件，使用默认配置、环境变量和命令行参数")
	}
}

func chineseHelpTemplate() string {
	return `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`
}

func chineseUsageTemplate() string {
	return `用法:
  {{.UseLine}}{{if .HasAvailableSubCommands}}

可用命令:
{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

参数:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

全局参数:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

示例:
{{.Example}}{{end}}

使用 "{{.CommandPath}} [command] --help" 查看子命令帮助。
`
}
