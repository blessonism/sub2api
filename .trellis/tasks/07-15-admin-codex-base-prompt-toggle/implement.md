# 实施计划

1. 在后端设置常量、DTO、解析/写入、运行时缓存和设置视图中加入开关及自定义正文。
2. 将 OpenAI OAuth 转换链路接入运行时设置：默认启用；自定义正文非空时覆盖内嵌默认选择；关闭时跳过自动补齐。
3. 在管理员设置页面及中英文 locale 增加开关、说明和多行 Prompt 编辑控件，并接入现有保存回显流程。
4. 增加后端设置解析/转换测试和前端 SettingsView 测试，覆盖默认值、关闭、正文覆盖、清空恢复和保存回显。
5. 运行后端相关 Go 测试、前端设置测试和必要的类型检查；检查现有未提交改动不被触碰。

## 质量门

- `go test ./backend/internal/service ./backend/internal/handler/admin ./backend/internal/pkg/openai`
- 前端 SettingsView 定向测试及项目既有类型检查命令
- `git diff --check`
