# LedgerBot - Intelligent Ledger Bot for Feishu

A smart ledger bot built for Feishu platform that makes personal finance tracking effortless through natural language processing and AI-powered categorization.

## Key Features

🤖 **AI-Powered Intelligence**: Leverages OpenAI GPT models to automatically recognize and categorize transactions from natural language input

💬 **Natural Language Friendly**: Supports various natural language expressions - no need to remember specific formats. Just chat naturally!

📊 **Feishu Bitable Integration**: Seamlessly stores all transaction data directly in Feishu Bitable (multi-dimensional tables)

🏷️ **Smart Categorization**: AI automatically recommends categories based on transaction descriptions - no manual selection needed

🔍 **Intelligent Querying**: Query transactions by time ranges using natural language (today, this week, last month, custom dates, etc.)

✏️ **Record Management**: Update and delete existing transactions easily with natural language commands

👤 **User Management**: Support for user mapping and renaming

📝 **Complete Audit Trail**: Preserves original user messages for full transaction history

## Use Cases

- Personal expense tracking
- Family budget management
- Team expense recording
- Financial data analysis

## Quick Start

```bash
docker run -d \
  -e FEISHU_APP_ID=your_app_id \
  -e FEISHU_APP_SECRET=your_app_secret \
  -e FEISHU_BITABLE_URL=your_bitable_url \
  -e AI_API_KEY=your_openai_api_key \
  -p 3906:3906 \
  ledgerbot:latest
```

## Natural Language Examples

**Recording transactions:**
- "Lunch cost 30 yuan"
- "Spent 45 yuan on taxi today"
- "Received 500 yuan salary"

**Querying:**
- "Show today's transactions"
- "Query this week's top 10 expenses"
- "Display last month's records"

**Updating:**
- "Update record recxxx amount to 1998"
- "Change record recxxx description to buying computer"

**Deleting:**
- "Delete record recxxx"

The AI understands your intent automatically - no need to memorize commands!

## Technology Stack

- Go 1.21+
- Feishu OpenAPI SDK
- OpenAI GPT Models
- Feishu Bitable

## Documentation

For detailed setup instructions, configuration options, and usage examples, please visit the [GitHub repository](https://github.com/yourusername/LedgerBot).

## License

MIT License

