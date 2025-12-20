# LedgerBot 飞书记账机器人 - 文件架构说明

## 🚀 执行流程概览

`main.go` → 配置加载 → 服务初始化 → HTTP服务器启动 → 飞书webhook → 消息处理 → AI解析 → 记账/重命名

## 📁 文件详解

### 1. **main.go** - 应用入口
- **作用**：程序启动和服务器初始化
- **流程**：
  1. 加载环境变量配置
  2. 初始化用户映射仓库（本地JSON文件）
  3. 初始化账单内存仓库
  4. 创建飞书服务（用于API调用）
  5. 创建AI服务（OpenAI工具调用模式）
  6. 启动HTTP服务器监听webhook
  7. 优雅关闭处理

### 2. **config/config.go** - 配置管理
- **作用**：统一管理系统配置文件
- **核心配置**：
  - 飞书AppID、AppSecret、多维表格配置
  - AI服务的BaseURL、API Key、模型配置
  - 用户映射文件路径、数据目录、日志级别
  - 缓存TTL、服务器端口等

---

## 🧠 AI处理流程

### 3. **internal/infrastructure/ai/openai_service.go** - AI核心
- **作用**：AI工具调用，完全通过OpenAI API的function calling模式处理
- **关键特性**：
  - `Execute()`接收用户输入，自动识别是记账还是重命名
  - `record_transaction`函数：记账功能（支持expense/income）
  - `rename_user`函数：用户重命名功能
  - 完全通过AI判断，无手动字符串匹配
- **执行方式**：智能识别中文自然语言

### 4. **internal/domain/ai.go** - AI领域模型
- **作用**：定义AI服务接口和数据结构
- **核心内容**：
  - `AIRequest/AIResponse`消息格式
  - `FunctionCall/Choice`函数调用解析
  - `BillExtraction`记账数据提取

---

## 💾 数据存储

### 5. **internal/infrastructure/repository/user_mapping_repo.go** - 用户映射
- **作用**：本地JSON文件存储多平台用户ID映射
- **功能**：
  - platform+platform_id → user_id+user_name
  - 支持飞书、微信、QQ等多平台统一用户管理
  - 持久化到JSON文件，程序重启不丢失

### 6. **internal/infrastructure/repository/bill_repo.go** - 账单数据
- **作用**：账单数据仓库（当前为内存实现）
  - **TODO**：可扩展为Feishu多维表格、SQLite、MySQL等
  - 提供CRUD操作、分类统计、月度汇总等功能

---

## 🏗️ 领域层

### 7. **internal/domain/**
- **user.go**：用户和平台枚举定义
- **bill.go**：账单模型、类型定义、仓库接口
- **ai.go**：AI领域模型
- **command.go**：AI命令解析接口

---

## 🎯 应用层

### 8. **internal/usecase/**
- **bill_usecase.go**：账单业务逻辑
  - 创建账单、智能分类、月度统计等
- **user_usecase.go**：用户相关操作
- **command_executor.go**：命令执行器（预留）

---

## 🤖 平台集成

### 9. **internal/infrastructure/platform/feishu/feishu_service.go** - 飞书集成
- **功能**：
  - 获取AccessToken（自动刷新）
  - 发送消息给用户
  - 获取用户信息
  - 添加记录到多维表格（如使用）

### 10. **internal/interfaces/http/handler/feishu_handler.go** - 飞书webhook处理
- **旧的处理逻辑**（建议重构）：
  - 手动解析是否为记账或重命名
  - 不使用AI工具调用

### 11. **internal/interfaces/http/handler/feishu_ai_tool_handler.go** - 新的AI集成
- **功能**：
  - 连接飞书webhook到AI服务
  - 使用 `Execute()` 方法调用AI工具
  - 自动生成回复发给用户

---

## 🔧 工具类

### 12. **pkg/**
- **logger/logger.go**：日志工具
- **cache/cache.go**：本地文件缓存实现

---

## 📝 配置和文档

- **.env.example**：环境变量配置模板
- **README.md**：项目说明文档
- **Multi-platformUserMapping.md**：多平台用户映射设计

---

## 🔄 理想执行流程（AI工具模式）

1. 用户发送消息 → Feishu webhook → `feishu_ai_tool_handler.go`
2. Handler获取用户映射信息（ID↔PlatformID↔Name）
3. 调用 `ai.Execute()` 进行AI分析
4. AI自动识别是记账还是重命名
5. 如果是记账：`record_transaction`函数被调用 → 创建账单
6. 如果是重命名：`rename_user`函数被调用 → 更新用户名称
7. 返回中文回复给Handler
8. Handler通过飞书服务发送回复给用户

---

## 💡 主要改进点

✅ **AI驱动**：完全使用function calling，无手动意图识别
✅ **多平台友好**：支持用户映射统一管理
✅ **自然语言**：支持中文自然语言输入
✅ **智能分类**：AI根据上下文自动判断支出/收入/分类

---

## 🔮 后续扩展建议

1. **数据持久化**：从内存仓库改成多维表格/SQL数据库
2. **更多AI功能**：月报表生成、消费分析、预算提醒等
3. **多平台支持**：微信、QQ集成
4. **可视化**：Web管理界面、报表图表
5. **高级功能**：多币种、转账记录、共享账本等