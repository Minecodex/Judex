# 报告格式

report.json（progress/delivery 通用）：

```json
{
  "text": "完成列表筛选；附件包含本地验证记录。",
  "expectedTaskVersion": 4,
  "codeRefs": [{"repositoryId":"<uuid>","branch":"task/filter","commit":"<full-sha>","verification":"reported"}]
}
```

- `verification` 只能写 `reported`（客户端声明）；平台固定显示"reported"，不允许自称 verified。
- 交付（delivery）必须引用 ready 材料：先 `material upload`，把返回的 versionId 放入 submission 的 materialVersionIds。
- 进度是提示不是验收；不要用 progress 代替交付。
