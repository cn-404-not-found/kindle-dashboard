# 项目规范与 AI 行为准则

- 技术栈：Go 1.25（module `kindle-dashboard`）。核心依赖：`github.com/fogleman/gg`（2D 图形绘制）、`golang.org/x/image`、`github.com/golang/freetype`（字体渲染）。这是一个用于生成 Kindle 仪表盘静态图像的命令行程序（main.go 入口 + render.go 渲染逻辑）。
- 编码风格：必须包含单元测试，保持模块化与清晰注释。
- 自动化：我不会手动修改代码，所有代码变更与测试均由 AI 执行。
