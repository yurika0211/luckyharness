# LuckyHarness v0.55.1 配置迁移指南

## 📋 变更说明

从 **v0.55.1** 开始，LuckyHarness 统一使用 **JSON** 格式作为配置文件格式，不再使用 YAML。

### 为什么迁移？

1. **统一格式**：与 `config.json`（channels/providers 配置）保持一致
2. **减少依赖**：移除对 `gopkg.in/yaml.v3` 的依赖
3. **简化代码**：JSON 是 Go 原生支持，无需额外解析库
4. **避免混淆**：单一配置文件格式，降低用户困惑

## 🔧 迁移步骤

### 方法 1：自动迁移（推荐）

```bash
# 使用迁移脚本
python3 /root/luckyharness-src/scripts/migrate_config.py

# 输出示例：
# 🔧 LuckyHarness 配置迁移工具 v0.55.1
# ==================================================
# 📖 读取 /root/.luckyharness/config.yaml...
# 💾 备份 /root/.luckyharness/config.yaml → /root/.luckyharness/config.yaml.bak...
# ✍️  写入 /root/.luckyharness/config.json...
# ✅ 迁移成功！
```

### 方法 2：手动迁移

```bash
# 1. 备份原配置
cp ~/.luckyharness/config.yaml ~/.luckyharness/config.yaml.bak

# 2. 使用 Python 转换
python3 -c "
import yaml, json
with open('~/.luckyharness/config.yaml') as f:
    config = yaml.safe_load(f)
with open('~/.luckyharness/config.json', 'w') as f:
    json.dump(config, f, indent=2, ensure_ascii=False)
"

# 3. 验证配置
cat ~/.luckyharness/config.json
```

### 方法 3：手动创建

如果原配置文件不存在或已损坏，可以手动创建：

```bash
cat > ~/.luckyharness/config.json << 'EOF'
{
  "provider": "openai",
  "api_key": "sk-YOUR_API_KEY",
  "api_base": "https://api.boaiak.com/v1",
  "model": "gpt-5.4-mini",
  "soul_path": "/root/.luckyharness/SOUL.md",
  "max_tokens": 4096,
  "temperature": 0.7,
  "memory": {
    "short_term_max_turns": 10,
    "midterm_expire_days": 90,
    "midterm_max_summaries": 100
  }
}
EOF
```

## 📝 配置项对照表

| YAML 字段 | JSON 字段 | 说明 |
|----------|----------|------|
| `provider` | `provider` | 提供商名称 |
| `api_key` | `api_key` | API 密钥 |
| `api_base` | `api_base` | API 基础地址 |
| `model` | `model` | 默认模型 |
| `soul_path` | `soul_path` | SOUL 文件路径 |
| `max_tokens` | `max_tokens` | 最大 token 数 |
| `temperature` | `temperature` | 温度参数 |
| `fallbacks` | `fallbacks` | 降级链配置 |
| `web_search` | `web_search` | Web 搜索配置 |
| `stream_mode` | `stream_mode` | 流式模式 |
| `memory` | `memory` | 记忆系统配置 |
| `model_router` | `model_router` | 模型路由配置 |

## ✅ 验证迁移

```bash
# 1. 检查文件是否存在
ls -lh ~/.luckyharness/config.json

# 2. 验证 JSON 格式
python3 -m json.tool ~/.luckyharness/config.json

# 3. 重启 LuckyHarness
lh version

# 4. 测试功能
lh chat "你好"
```

## 🔄 回滚方案

如果需要回滚到 YAML 版本：

```bash
# 1. 停止 LuckyHarness
pkill -f "lh msg-gateway"

# 2. 恢复旧二进制
cp ~/.luckyharness/lh.bak.v0.55.0 ~/.luckyharness/lh

# 3. 恢复旧配置
mv ~/.luckyharness/config.yaml.bak ~/.luckyharness/config.yaml

# 4. 重启
~/.luckyharness/lh msg-gateway start --platform telegram --token "YOUR_TOKEN"
```

## 📊 迁移影响

| 项目 | 迁移前 | 迁移后 |
|------|--------|--------|
| **配置文件** | `config.yaml` | `config.json` |
| **格式** | YAML | JSON |
| **依赖** | `gopkg.in/yaml.v3` | 无（Go 原生） |
| **版本** | v0.55.0 及之前 | v0.55.1+ |

## ❓ 常见问题

### Q: 迁移后原来的 `config.yaml` 还在吗？
A: 在的，迁移脚本会将其备份为 `config.yaml.bak`。

### Q: 可以手动编辑 `config.json` 吗？
A: 可以，但建议使用 `lh config set <key> <value>` 命令。

### Q: 迁移后需要重启吗？
A: 是的，需要重启 LuckyHarness 网关才能加载新配置。

### Q: 如果迁移失败怎么办？
A: 迁移脚本会保留原文件，可以手动创建 `config.json` 或回滚。

## 📚 相关文档

- [配置管理文档](docs/config.md)
- [配置项说明](docs/config-reference.md)
- [故障排查](docs/troubleshooting.md)
