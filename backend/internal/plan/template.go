package plan

const DefaultTemplate = `# PLAN

> 说明：
> - 每个任务一行：- [ ] ... <!-- id: ... -->
> - scope 为可选 glob 列表（相对 workspace 根目录），用于限制可写范围
> - acceptance 为验收标准（observer 仅做只读文件/内容校验；不执行命令）

## 1. Example
- [ ] 填写任务描述（并补充 scope/acceptance） <!-- id: 1 -->
  - scope:
    - backend/**
  - acceptance:
    - files:
      - backend/README.md
    - must_contain:
      - backend/README.md: "TODO"
`
