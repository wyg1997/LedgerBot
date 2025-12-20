# 快速开始指南

## 🚀 项目启动步骤

### 1. 环境准备
```bash
# 安装Go（需要Go 1.19及以上）
go version

# 克隆项目
git clone https://github.com/your-repo/LedgerBot.git
cd LedgerBot
```

### 2. 配置设置
```bash
# 复制环境变量模板
cp .env.example .env

# 编辑配置文件
vim .env
```

**必需配置**：
- `FEISHU_APP_ID=你的飞书应用ID`
- `FEISHU_APP_SECRET=你的飞书应用密钥`
- `AI_API_KEY=OpenAI或兼容API的密钥`

### 3. 编译运行
```bash
# 下载依赖
go mod download

# 编译（可选）
go build -o ledger-bot

# 运行
go run main.go
# 或 ./ledger-bot （如果编译了）
```

### 4. 飞书配置

1. **创建飞书应用**：
   - 访问 https://open.feishu.cn/
   - 创建自建应用
   - 获取AppID和AppSecret

2. **配置权限**：
   - 添加 "获取用户基本信息" 权限
   - 添加 "获取群组消息" 权限（如需要群聊支持）

3. **配置Webhook**：
   - 设置Webhook URL：`http://your-domain/webhook/feishu`
   - 选择接收的消息类型（默认接收消息事件）

### 5. 使用机器人

**添加机器人到飞书**：
- 直接@机器人或在私聊中发送消息

**记账示例**：
```
午饭 25.5          # AI自动识别为支出
奶茶 -15           # 加上减号表示支出（可选）
工资 +5000         # 加上加号表示收入
叫我张三           # 设置自己的昵称
```

## 📋 目录结构

```
LedgerBot/
├── config/                          # 配置管理
│   └── config.go                    # 环境变量加载和配置结构体
├── internal/                        # 核心业务逻辑（不暴露给外部）
│   ├── domain/                      # 领域层 - 核心业务概念
│   │   ├── ai.go                   # AI领域模型
│   │   ├── bill.go                 # 账单领域模型
│   │   ├── command.go              # 命令领域模型
│   │   └── user.go                 # 用户领域模型
│   ├── infrastructure/              # 基础设施层 - 外部集成
│   │   ├── ai/                     # AI服务实现
│   │   │   └── openai_service.go   # OpenAI功能调用服务
│   │   ├── platform/               # 平台集成
│   │   │   └── feishu/            # 飞书API服务
│   │   │       └── feishu_service.go
│   │   └── repository/            # 数据持久化
│   │       ├── bill_repo.go        # 账单数据仓库
│   │       └── user_mapping_repo.go # 用户映射本地存储
│   ├── usecase/                    # 应用层 - 业务逻辑用例
│   │   ├── bill_usecase.go         # 账单业务逻辑
│   │   ├── command_executor.go     # 命令执行器
│   │   └── user_usecase.go         # 用户业务逻辑
│   └── interfaces/                # 接口层 - 外部接口
│       └── http/
│           └── handler/           # 请求处理器
│               ├── feishu_handler.go # 飞书webhook旧版（不推荐）
│               └── feishu_ai_tool_handler.go # 新版本AI工具模式
├── pkg/                            # 通用工具包
│   ├── cache/                     # 缓存实现
│   │   └── cache.go               # 本地文件持久化缓存
│   └── logger/                    # 日志工具
│       └── logger.go              # 日志管理
├── main.go                        # 程序入口
├── go.mod                         # Go模块文件
├── .env.example                   # 环境变量模板
├── README.md                      # 项目说明
└── docs/                          # 文档目录
    ├── arch.md                    # 架构文档（本文件）
    └── quick-start.md             # 快速开始
```

## 🎉 恭喜你！

你已经成功运行了一个智能记账机器人！它支持：
- 🤖 AI自然语言理解和分类
- 🧠 智能判断支出/收入
- 🔄 多平台用户映射
- 📊 本地数据存储