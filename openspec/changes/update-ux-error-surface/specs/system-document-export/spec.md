## ADDED Requirements

### Requirement: Document export UI MUST make the primary action obvious
系统必须 (MUST) 在文档导出页面突出主操作（导出），并确保工作区“选择”按钮语义明确为“选择文件夹”，避免误解为提交执行。

#### Scenario: Export call-to-action is clear
- **GIVEN** 用户打开文档导出页面
- **WHEN** 页面渲染完成
- **THEN** “导出”是视觉上最突出的主按钮（best-effort）
- **AND** workspace chooser 的文案/样式明确其为“选择文件夹”（best-effort）

