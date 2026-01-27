## MODIFIED Requirements
### Requirement: `write_file` overwrite 必须是原子替换（Crash Consistency）
系统必须 (MUST) 确保 `write_file` 在 overwrite 模式下采用原子替换策略（同目录写临时文件 + rename 覆盖目标文件），以避免进程崩溃/中断导致半写文件。

系统可以 (MAY) 在 append 模式保持现有语义（append 不要求原子替换），但应在输出中清晰标注写入模式与写入统计信息，便于验收与复核。

#### Scenario: overwrite 在崩溃中断时不污染目标文件
- **GIVEN** 目标文件 `a.txt` 已存在且内容为 `old`
- **WHEN** 系统执行 overwrite 写入，但在 rename 之前发生异常中断
- **THEN** `a.txt` 仍保持 `old`（不会出现半写内容）

### Requirement: SBE 写回必须复用原子替换
系统必须 (MUST) 确保 SBE（edit）对文件的写回同样使用原子替换策略，避免出现“edit 成功返回但文件损坏”的 silent corruption。

#### Scenario: edit 写回不产生半写文件
- **GIVEN** SBE 对某文件执行替换并准备写回
- **WHEN** 写回过程中发生异常中断
- **THEN** 原文件要么保持不变，要么被完整的新版本替换（不得出现半写状态）

