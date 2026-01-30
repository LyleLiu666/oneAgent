## ADDED Requirements

### Requirement: The system MUST normalize/repair tool arguments (best-effort)
当 provider/模型输出的 tool call arguments 不是严格合法 JSON 时，系统必须 (MUST) 做 best-effort 的 normalization/repair，以尽量将其修复为可 `json.Unmarshal` 的 JSON 字符串，减少无意义的 tool loop 浪费。

修复应至少覆盖（best-effort）：
- ```json 围栏（去除围栏后再校验）
- 前后夹杂解释文本（抽取最外层 JSON object/array）
- 整体为 JSON string 的情况（先 unquote 再解析）

#### Scenario: Code-fenced JSON arguments are repaired
- **GIVEN** tool arguments 形如 ```json { ... } ```
- **WHEN** 系统进行 arguments normalization
- **THEN** 得到合法 JSON 字符串（best-effort）

#### Scenario: Arguments with prefix/suffix text are repaired
- **GIVEN** tool arguments 形如 `Here is JSON: { ... } thanks`
- **WHEN** 系统进行 arguments normalization
- **THEN** 抽取并返回合法 JSON（best-effort）

#### Scenario: Quoted JSON arguments are repaired
- **GIVEN** tool arguments 是一个 JSON string（内容为 `{...}` 或 `[...]`）
- **WHEN** 系统进行 arguments normalization
- **THEN** 返回被解码后的合法 JSON（best-effort）

