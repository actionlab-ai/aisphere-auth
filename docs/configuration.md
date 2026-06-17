# 配置加载与命令行设计

`aisphere-auth` 使用 Cobra + Viper 管理命令行、配置文件和环境变量。

## 配置优先级

```text
命令行参数 > 环境变量 > 配置文件 > 默认值
```

这意味着：

1. 生产环境可以主要使用 `configs/config.yaml`。
2. 密码、clientSecret、serviceToken、jwtSecret 等敏感项可以通过环境变量或 Secret 注入。
3. 临时调试可以通过命令行参数覆盖配置文件。

## 常用命令

查看中文帮助：

```bash
./aisphere-auth -h
```

启动服务：

```bash
./aisphere-auth --config configs/config.yaml
```

检查配置但不启动服务：

```bash
./aisphere-auth check-config --config configs/config.yaml
```

打印最终合并后的脱敏配置：

```bash
./aisphere-auth --config configs/config.yaml --print-config
```

查看版本：

```bash
./aisphere-auth version
```

## 常用命令行覆盖参数

```bash
./aisphere-auth \
  --config configs/config.yaml \
  --addr :18080 \
  --mode release \
  --session-provider redis \
  --redis-addrs 127.0.0.1:6379 \
  --casdoor-endpoint http://127.0.0.1:8000 \
  --casdoor-client-id xxx \
  --casdoor-client-secret xxx \
  --service-token xxx \
  --service-token-required=true
```

## 配置文件

样例文件：

```text
configs/config.yaml.example
```

复制为本地配置：

```bash
cp configs/config.yaml.example configs/config.yaml
```

`configs/config.yaml` 已在 `.gitignore` 中忽略，不应提交真实密钥。

## 环境变量兼容

为了兼容早期脚本，仍然支持原来的环境变量命名，例如：

```bash
AISPHERE_AUTH_ADDR=:18080
AISPHERE_SESSION_PROVIDER=redis
AISPHERE_REDIS_ADDRS=127.0.0.1:6379
AISPHERE_CASDOOR_ENDPOINT=http://127.0.0.1:8000
AISPHERE_CASDOOR_CLIENT_ID=xxx
AISPHERE_CASDOOR_CLIENT_SECRET=xxx
AISPHERE_SERVICE_TOKEN=xxx
```

## 生产建议

生产环境建议：

```yaml
server:
  mode: "release"

session:
  provider: "redis"

gateway:
  cookieSecure: true

internal:
  serviceTokenRequired: true

authz:
  failClosed: true
  cacheTTLSeconds: 30
```

敏感项建议通过 Secret 注入：

```bash
AISPHERE_CASDOOR_CLIENT_SECRET
AISPHERE_SERVICE_TOKEN
AISPHERE_JWT_SECRET
AISPHERE_REDIS_PASSWORD
```
