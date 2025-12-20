# LedgerBot - 飞书记账机器人

一个基于飞书平台和多维表格的智能记账机器人，支持自然语言输入和AI自动分类。

## 功能特性

- 🤖 **AI智能识别**: 使用OpenAI GPT模型自动识别账单信息
- 💬 **自然语言输入**: 支持自然语言输入，无需特定格式
- 📊 **多维表格存储**: 数据直接存储在飞书多维表格中
- 🏷️ **智能分类**: AI根据描述自动推荐分类
- 👤 **用户管理**: 支持飞书用户映射和重命名
- 📝 **原始消息记录**: 保存用户输入的完整原始消息
- ⏰ **精确时间**: 记录账单创建的具体时间（包含时分秒）

## 快速开始

### 1. 环境配置

复制环境变量模板并编辑：
```bash
cp .env.example .env
# 然后编辑.env文件填写你的配置
```

### 2. 获取多维表格URL

1. 在飞书打开你的多维表格
2. 复制浏览器地址栏的完整URL
3. URL格式应该类似于：`https://example.feishu.cn/base/APP_TOKEN?table=TABLE_TOKEN`

### 3. 多维表格字段设置

在你的飞书多维表格中创建以下字段（可根据需要自定义字段名）：

1. **描述** (默认字段名：描述) - 单行文本
   - 存储账单的描述信息，如"午饭"、"打车"等

2. **金额** (默认字段名：金额) - 金额或数字类型
   - 存储账单金额数值

3. **类型** (默认字段名：类型) - 单选类型
   - 选项：收入/支出
   - 通过AI自动判断是收入还是支出

4. **分类** (默认字段名：分类) - 单行文本或单选类型
   - AI根据描述自动推荐的分类，如"餐饮"、"交通"等

5. **日期** (默认字段名：日期) - 日期时间类型
   - 格式：YYYY-MM-DD HH:MM:SS（如：2024-12-17 14:30:25）
   - 精确到秒，记录账单创建的时间

6. **记录者** (默认字段名：记录者) - 单行文本
   - 存储用户的显示名称

7. **原始消息** (默认字段名：原始消息) - 单行文本
   - 存储用户输入的完整原始消息，如"午饭花了30块"

### 4. 获取飞书应用配置

1. 登录[飞书开发者后台](https://open.feishu.cn/)
2. 创建企业自建应用
3. 获取App ID和App Secret
4. 添加以下权限：
   - 获取用户联系方式
   - 发送消息
   - 编辑多维表格

### 5. 运行机器人

```bash
go mod tidy
go run main.go
```

## 配置说明

### 必需的环境变量

```bash
# 飞书应用
FEISHU_APP_ID=你的app_id
FEISHU_APP_SECRET=你的app_secret
FEISHU_BOT_NAME=记账管家  # Bot名称，用于识别@提及（可选，默认为"记账管家"）

# 只需复制完整的飞书多维表格URL！
FEISHU_BITABLE_URL=https://example.feishu.cn/base/YOUR_APP_TOKEN?table=YOUR_TABLE_TOKEN

# AI配置
AI_API_KEY=你的openai_api_key
```

### 如何使用多维表格URL

1. 在飞书中打开你的多维表格
2. 复制浏览器地址栏的完整URL，像这样：
   ```
   https://example.feishu.cn/base/VLqDbQT7LaW4vQsccA5cPIVQnOf?table=tblxI2XhCqNJ7VCI
   ```
3. 直接粘贴这个URL到配置文件即可！

### 使用示例

在飞书中，直接发送以下格式的消息给机器人：
- "午饭30元"
- "打车花了45块去参加聚会"
- "今天收入500元工资"

机器人处理结果：
- **记录到表格中的数据**：
  - 描述：午饭
  - 金额：30
  - 类型：支出
  - 分类：餐饮（AI推荐）
  - 日期：2024-12-17 14:30:25
  - 记录者：张三
  - 原始消息：午饭30元

### 用户重命名

发送："叫我小明"
- 机器人会将你的显示名称改为"小明"
- 之后所有账单记录的用户名将显示为"小明"

## API接口

- `POST /webhook/feishu` - 飞书Webhook接口
- `GET /health` - 健康检查

## 自定义字段名

如果你的多维表格使用了不同的字段名，可以通过环境变量自定义：

```env
# 字段映射配置
FEISHU_FIELD_DESCRIPTION=描述
FEISHU_FIELD_AMOUNT=金额
FEISHU_FIELD_TYPE=类型
FEISHU_FIELD_CATEGORY=分类
FEISHU_FIELD_DATE=日期
FEISHU_FIELD_USER_NAME=记录者
FEISHU_FIELD_ORIGINAL_MSG=原始消息
```

## 环境变量配置（完整参考）

| 变量名 | 说明 | 默认值 |
|--------|------|----------|
| FEISHU_APP_ID | 飞书应用ID | 必填 |
| FEISHU_APP_SECRET | 飞书应用密钥 | 必填 |
| FEISHU_BOT_NAME | Bot名称，用于识别@提及 | 记账管家 |
| FEISHU_BITABLE_URL | 飞书多维表格完整URL | 必填 |
| AI_API_KEY | OpenAI API密钥 | 必填 |
| SERVER_PORT | 服务端口号 | 8080 |
| USER_MAPPING_FILE | 用户映射文件路径 | ./data/user_mapping.json |
| LOG_LEVEL | 日志级别 | info |

## 直接通过环境变量运行

你也可以不使用.env文件，直接设置环境变量：
```bash
export FEISHU_APP_ID="your_app_id"
export FEISHU_APP_SECRET="your_app_secret"
export FEISHU_BITABLE_URL="https://example.feishu.cn/base/APP_TOKEN?table=TABLE_TOKEN"
export AI_API_KEY="your_openai_api_key"
go run main.go
```

## License

MIT License