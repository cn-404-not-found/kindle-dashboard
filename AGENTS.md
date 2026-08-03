# 项目编码规范与 AI 行为准则

- 架构与语言：Go 1.25（module `kindle-dashboard`）。核心依赖：`github.com/fogleman/gg`（2D 图形绘制）、`golang.org/x/image`、`github.com/golang/freetype`（字体渲染）。用于生成 Kindle 仪表盘静态图像的命令行程序（main.go 入口 + render.go 渲染逻辑）。
- 代码风格：模块化开发，函数需包含清晰注释，严禁修改无关代码。
- 测试要求：每次修改核心逻辑后，必须运行或补充对应的单元测试。
- 开发模式：用户不手动编辑代码，所有变更由 AI 自动完成。
